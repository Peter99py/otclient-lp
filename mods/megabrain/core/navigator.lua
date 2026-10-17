MegaBrain.Navigator = {}

local exploring = false
local routeMode = false
local paused = false
local lastCavebotStatus = nil

-- Estado da rota ativa (recebida do backend via resposta de /tiles).
local routeId = 0
local routePath = nil
local routeDirs = nil
local routeCursor = 1
local routeLoopStart = 1
local routeFetching = false
local routeRetryAt = 0

-- Anti-stuck (detecção e recuperação de travamento)
local lastPos = nil
local lastMoveTime = 0
local lastCommandTime = 0
local stuckAttempts = 0
local STUCK_TIMEOUT_MS = 2000 -- Tempo parado tentando andar antes de acionar anti-stuck

-- Caminho (tiles) comandado no último tick; enviado ao backend para a UI.
local lastCommand = nil

-- Auto-rota: dispara uma vez por sessão de exploração.
local autoRouteTried = false

-- Callback opcional: chamado quando a exploração conclui e a rota é ativada.
MegaBrain.Navigator.onExploreDone = nil

-- Direções Otc::Direction: N=0, E=1, S=2, W=3.
local DIR_DX = { [0] = 0, [1] = 1, [2] = 0, [3] = -1 }
local DIR_DY = { [0] = -1, [1] = 0, [2] = 1, [3] = 0 }

-- Constrói a lista de tiles (formato [[x,y],...]) percorrida a partir de pos.
local function commandTiles(pos, dirs)
    local out = { { pos.x, pos.y } }
    local x, y = pos.x, pos.y
    for _, d in ipairs(dirs) do
        x = x + (DIR_DX[d] or 0)
        y = y + (DIR_DY[d] or 0)
        table.insert(out, { x, y })
    end
    return out
end

function MegaBrain.Navigator.getCommand()
    return lastCommand
end

function MegaBrain.Navigator.setExploring(on)
    exploring = on
    if on then
        autoRouteTried = false
    end
end

function MegaBrain.Navigator.setRouteMode(on)
    routeMode = on
end

-- Modo atual para a UI web: "route" | "explore" | "off".
function MegaBrain.Navigator.getMode()
    if routeMode then
        return "route"
    elseif exploring then
        return "explore"
    end
    return "off"
end

-- O cavebot (aba Target) publica o status em modules.game_bot.targetStatus.
-- O MegaBrain só anda quando o cavebot está ocioso (Waiting/Off/On).
local function cavebotStatus()
    local gb = modules and modules.game_bot
    return gb and gb.targetStatus
end

local function cavebotAllows()
    local status = cavebotStatus()
    if not status then
        return true
    end
    return status == "Waiting" or status == "Off" or status == "On"
end

-- Loga cada mudança de status do cavebot (ajuda a depurar a pausa).
local function logCavebotStatus()
    local status = cavebotStatus()
    if status ~= lastCavebotStatus then
        lastCavebotStatus = status
        MegaBrain.Logger.debugLog("Cavebot status: " .. tostring(status))
    end
end

-- Busca a rota já ajustada para começar no ponto mais próximo do jogador
-- (caminho de entrada + rota rotacionada).
local function fetchRoute(id)
    if routeFetching then
        return
    end
    routeFetching = true
    HTTP.getJSON(MegaBrain.Config.routeUrl .. "/entry?id=" .. id, function(data, err)
        routeFetching = false
        if err or not data or not data.directions then
            MegaBrain.Logger.debugLog("Falha ao buscar rota: " .. tostring(err))
            return
        end
        routePath = data.path
        routeDirs = data.directions
        routeCursor = 1
        routeLoopStart = data.loop_start or 1
        stuckAttempts = 0
        lastPos = nil
        lastMoveTime = g_clock.millis()
        MegaBrain.Logger.debugLog("Rota " .. id .. " carregada: " .. #routeDirs .. " passos")
    end)
end

-- Recebe o estado do backend (paused/monsterCount/routeId).
function MegaBrain.Navigator.onState(state)
    local nowPaused = state and state.paused or false
    if nowPaused and not paused then
        g_game.stop()
        lastCommand = nil
        MegaBrain.Logger.debugLog("Pausado: " .. tostring(state.monsterCount) .. " monstros alcancaveis")
    elseif not nowPaused and paused then
        MegaBrain.Logger.debugLog("Retomando")
    end
    paused = nowPaused

    local rid = state and state.routeId or 0
    if rid ~= routeId then
        routeId = rid
        routePath = nil
        routeDirs = nil
        routeCursor = 1
        routeLoopStart = 1
        routeRetryAt = 0
        stuckAttempts = 0
        lastPos = nil
    end
end

-- Encontra o índice do tile atual na rota, priorizando a vizinhança do cursor.
local function findCursor(pos)
    if not routePath then
        return nil
    end
    local limit = math.min(routeCursor + 12, #routePath)
    for i = routeCursor, limit do
        local t = routePath[i]
        if t[1] == pos.x and t[2] == pos.y then
            return i
        end
    end
    local best, bestD
    for i = 1, #routePath do
        local t = routePath[i]
        if t[1] == pos.x and t[2] == pos.y then
            local d = math.abs(i - routeCursor)
            if not bestD or d < bestD then
                best, bestD = i, d
            end
        end
    end
    return best
end

-- Encontra o índice do tile da rota mais próximo da coordenada pos,
-- que esteja a partir de startIndex (para permitir avançar após um ponto bloqueado).
local function findNearestRouteIndex(pos, startIndex, minDistance)
    if not routePath or #routePath == 0 then
        return nil
    end
    startIndex = math.max(1, startIndex or 1)
    minDistance = minDistance or 1
    local bestIdx = nil
    local bestDist = math.huge

    for i = startIndex, #routePath do
        local t = routePath[i]
        local d = math.max(math.abs(t[1] - pos.x), math.abs(t[2] - pos.y))
        if d >= minDistance and d < bestDist then
            bestDist = d
            bestIdx = i
        end
    end

    -- Se não encontrou do startIndex para frente, faz wrap para o início da rota
    if not bestIdx and startIndex > 1 then
        for i = 1, startIndex - 1 do
            local t = routePath[i]
            local d = math.max(math.abs(t[1] - pos.x), math.abs(t[2] - pos.y))
            if d >= minDistance and d < bestDist then
                bestDist = d
                bestIdx = i
            end
        end
    end

    return bestIdx, bestDist
end

-- Tenta recuperar o personagem enviando autoWalk com pathfinding para um ponto alvo da rota
local function tryRecoverStuck(localPlayer, pos)
    stuckAttempts = stuckAttempts + 1
    local targetIdx = nil

    if stuckAttempts == 1 then
        -- Tentativa 1: Ponto mais próximo da rota (à frente do cursor)
        targetIdx = findNearestRouteIndex(pos, routeCursor, 1)
        MegaBrain.Logger.debugLog(string.format("Anti-stuck [Tentativa 1]: Mirando no ponto mais proximo (idx %s)", tostring(targetIdx)))
    else
        -- Tentativas subsequentes: Pular para o ponto após o bloqueado (skip)
        local skipOffset = (stuckAttempts - 1) * 2
        local searchStart = (routeCursor + skipOffset)
        if searchStart > #routePath then
            searchStart = routeLoopStart
        end
        targetIdx = findNearestRouteIndex(pos, searchStart, 2)
        MegaBrain.Logger.debugLog(string.format("Anti-stuck [Tentativa %d]: Pulando para frente da rota (idx %s)", stuckAttempts, tostring(targetIdx)))
    end

    if targetIdx and routePath[targetIdx] then
        local targetTile = routePath[targetIdx]
        local destPos = { x = targetTile[1], y = targetTile[2], z = pos.z }
        routeCursor = targetIdx
        lastMoveTime = g_clock.millis() -- reinicia a janela para dar tempo de andar
        localPlayer:autoWalk(destPos)
        return true
    end

    return false
end

local function stepRoute()
    if paused or routeId == 0 then
        lastMoveTime = g_clock.millis()
        return
    end
    logCavebotStatus()
    if not cavebotAllows() then
        lastMoveTime = g_clock.millis()
        return
    end
    if not routePath then
        fetchRoute(routeId)
        return
    end

    local localPlayer = g_game.getLocalPlayer()
    if not localPlayer then
        return
    end

    local now = g_clock.millis()
    local pos = localPlayer:getPosition()

    -- Monitoramento de deslocamento
    if not lastPos or pos.x ~= lastPos.x or pos.y ~= lastPos.y or pos.z ~= lastPos.z then
        lastPos = { x = pos.x, y = pos.y, z = pos.z }
        lastMoveTime = now
        stuckAttempts = 0
    end

    if localPlayer:isWalking() then
        return
    end

    -- Se já enviamos comando mas o personagem ficou parado por mais de STUCK_TIMEOUT_MS
    if lastCommandTime > 0 and (now - lastMoveTime) >= STUCK_TIMEOUT_MS then
        MegaBrain.Logger.debugLog(string.format("Stuck detectado! Parado ha %d ms em (%d, %d, %d)", now - lastMoveTime, pos.x, pos.y, pos.z))
        if tryRecoverStuck(localPlayer, pos) then
            lastCommandTime = now
            return
        end
    end

    local idx = findCursor(pos)
    if not idx then
        -- Fora da rota: tenta caminhar ao ponto mais próximo ou recarrega entrada
        local nearestIdx = findNearestRouteIndex(pos, 1, 1)
        if nearestIdx and routePath[nearestIdx] then
            local targetTile = routePath[nearestIdx]
            routeCursor = nearestIdx
            localPlayer:autoWalk({ x = targetTile[1], y = targetTile[2], z = pos.z })
            lastCommandTime = now
            return
        end

        if now >= routeRetryAt then
            routeRetryAt = now + 3000
            routePath = nil
        end
        return
    end
    routeCursor = idx

    local dirs = {}
    for i = routeCursor, math.min(routeCursor + 4, #routeDirs) do
        table.insert(dirs, routeDirs[i])
    end
    if #dirs == 0 then
        routeCursor = routeLoopStart -- fim do loop: reinicia
        return
    end
    lastCommand = commandTiles(pos, dirs)
    lastCommandTime = now
    g_game.autoWalk(dirs, pos)
end

-- Exploração concluída: gera/ativa a rota da cave e avisa o client para
-- mudar para o modo rota (uma vez por sessão).
local function tryAutoRoute()
    if autoRouteTried then
        return
    end
    autoRouteTried = true
    if not MegaBrain.Config.autoRoute then
        return
    end
    HTTP.postJSON(MegaBrain.Config.routeUrl .. "/auto", {}, function(data, err)
        if err or not data or not data.ok or not data.routeId or data.routeId == 0 then
            local reason = err or (data and data.reason) or "sem rota"
            MegaBrain.Logger.debugLog("Auto-rota: não gerada (" .. tostring(reason) .. ")")
            return
        end
        MegaBrain.Logger.debugLog("Auto-rota: rota " .. data.routeId .. " ativada")
        if MegaBrain.Navigator.onExploreDone then
            MegaBrain.Navigator.onExploreDone()
        end
    end)
end

local function stepExplore()
    if not exploring or paused then
        return
    end

    logCavebotStatus()
    if not cavebotAllows() then
        return
    end

    local localPlayer = g_game.getLocalPlayer()
    if not localPlayer or localPlayer:isWalking() then
        return
    end

    HTTP.getJSON(MegaBrain.Config.commandUrl, function(data, err)
        if err or not data then
            return
        end
        if data.done then
            tryAutoRoute()
            return
        end
        if not data.directions or #data.directions == 0 then
            return
        end
        lastCommand = commandTiles(localPlayer:getPosition(), data.directions)
        g_game.autoWalk(data.directions, localPlayer:getPosition())
    end)
end

function MegaBrain.Navigator.step()
    if routeMode then
        stepRoute()
    else
        stepExplore()
    end
end

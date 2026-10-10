MegaBrain.Collector = {}

local function creatureKind(creature)
    if creature:isMonster() then
        return "monster"
    elseif creature:isNpc() then
        return "npc"
    elseif creature:isPlayer() then
        return "player"
    end
    return nil
end

-- Escada/buraco: cor do minimapa na faixa 210-213 (heurística dos bots).
local function isFloorChange(pos)
    local color = g_map.getMinimapColor(pos)
    return color >= 210 and color <= 213
end

function MegaBrain.Collector.collect()
    local localPlayer = g_game.getLocalPlayer()
    if not localPlayer then
        return nil
    end

    local playerPosition = localPlayer:getPosition()
    local z = playerPosition.z
    local radius = MegaBrain.Config.radius
    local tilesData = {}

    for x = playerPosition.x - radius, playerPosition.x + radius do
        for y = playerPosition.y - radius, playerPosition.y + radius do
            local tile = g_map.getTile({ x = x, y = y, z = z })
            if tile then
                table.insert(tilesData, {
                    x = x,
                    y = y,
                    z = z,
                    walkable = tile:isWalkable(true),
                    dynamic = tile:isWalkable(false),
                    floorChange = isFloorChange({ x = x, y = y, z = z }),
                })
            end
        end
    end

    local creaturesData = {}
    for _, creature in ipairs(g_map.getSpectators(playerPosition, false)) do
        if not creature:isLocalPlayer() then
            local pos = creature:getPosition()
            local kind = creatureKind(creature)
            if kind and pos.z == z then
                table.insert(creaturesData, {
                    x = pos.x,
                    y = pos.y,
                    z = pos.z,
                    kind = kind,
                })
            end
        end
    end

    return {
        player = { x = playerPosition.x, y = playerPosition.y, z = z },
        tiles = tilesData,
        creatures = creaturesData,
    }
end

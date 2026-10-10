MegaBrain.Backend = {}

local BIN_NAME = "megabrain-engine"

-- Onde o binário está embarcado no módulo.
local SOURCE_DIRS = {
    "/mods/megabrain/engine/",
}

-- Diretório de dados (db/port), virtual, dentro do write dir do client.
local DATA_VIRTUAL = "/megabrain"

local function isWindows()
    return g_platform.getOSName():lower():find("windows") ~= nil
end

local function binName()
    if isWindows() then
        return BIN_NAME .. ".exe"
    end
    return BIN_NAME
end

-- Caminho real (host) do diretório de dados.
local function dataDir()
    return g_resources.getWriteDir() .. DATA_VIRTUAL
end

-- Caminho real (host) do binário embarcado no módulo.
local function sourceBin()
    for _, dir in ipairs(SOURCE_DIRS) do
        local virtual = dir .. binName()
        local exists = g_resources.fileExists(virtual)
        MegaBrain.Logger.debugLog(string.format("Procurando binário em %s -> %s", virtual, tostring(exists)))
        if exists then
            return g_resources.getRealPath(virtual)
        end
    end
    return nil
end

local function readPort()
    local file = DATA_VIRTUAL .. "/port"
    if not g_resources.fileExists(file) then
        return nil
    end
    return tonumber(g_resources.readFileContents(file))
end

-- URL da interface web, ou nil se o backend não estiver rodando.
function MegaBrain.Backend.getUrl()
    if not g_platform.isProcessRunning(binName()) then
        return nil
    end
    local port = readPort()
    if not port then
        return nil
    end
    return "http://127.0.0.1:" .. port
end

function MegaBrain.Backend.start()
    MegaBrain.Logger.debugLog("Iniciando backend. OS=" .. g_platform.getOSName())
    local bin = sourceBin()
    if not bin then
        MegaBrain.Logger.debugLog("Binário do backend não encontrado no módulo.")
        return
    end

    g_resources.makeDir(DATA_VIRTUAL)
    local dir = dataDir()
    MegaBrain.Logger.debugLog("Binário: " .. bin)
    MegaBrain.Logger.debugLog("dataDir: " .. dir)

    local ok
    if isWindows() then
        ok = g_platform.spawnProcess(bin, { "-data-dir", dir })
    else
        -- Garante o bit de execução antes de rodar.
        local cmd = string.format("chmod +x '%s' && exec '%s' -data-dir '%s'", bin, bin, dir)
        MegaBrain.Logger.debugLog("Comando: " .. cmd)
        ok = g_platform.spawnProcess("/bin/sh", { "-c", cmd })
    end
    MegaBrain.Logger.debugLog("spawnProcess: " .. tostring(ok))
end

function MegaBrain.Backend.stop()
    local ok = g_platform.killProcess(binName())
    MegaBrain.Logger.debugLog("killProcess: " .. tostring(ok))
end

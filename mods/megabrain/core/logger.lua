MegaBrain.Logger = {}

local debugEnabled = false

function MegaBrain.Logger.setDebug(on)
    debugEnabled = on
end

function MegaBrain.Logger.debugLog(message)
    if debugEnabled then
        print("[MegaBrain] " .. tostring(message))
    end
end

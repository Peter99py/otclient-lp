MegaBrain.Persist = {}

local KEY = "megabrain"

function MegaBrain.Persist.load()
    local s = g_settings.getNode(KEY)
    if not s then
        return
    end
    if s.monsterStopLimit then
        MegaBrain.Config.monsterStopLimit = s.monsterStopLimit
    end
    if s.monsterResumeLimit then
        MegaBrain.Config.monsterResumeLimit = s.monsterResumeLimit
    end
    if s.resumeDelayMs then
        MegaBrain.Config.resumeDelayMs = s.resumeDelayMs
    end
    if s.autoRoute ~= nil then
        MegaBrain.Config.autoRoute = s.autoRoute
    end
    if s.routeSpacing then
        MegaBrain.Config.routeSpacing = s.routeSpacing
    end
end

function MegaBrain.Persist.save()
    local s = g_settings.getNode(KEY) or {}
    s.monsterStopLimit = MegaBrain.Config.monsterStopLimit
    s.monsterResumeLimit = MegaBrain.Config.monsterResumeLimit
    s.resumeDelayMs = MegaBrain.Config.resumeDelayMs
    s.autoRoute = MegaBrain.Config.autoRoute
    s.routeSpacing = MegaBrain.Config.routeSpacing
    g_settings.setNode(KEY, s)
    g_settings.save()
end

function MegaBrain.Persist.loadWindow()
    local s = g_settings.getNode(KEY)
    return s and s.window or nil
end

function MegaBrain.Persist.saveWindow(win)
    local s = g_settings.getNode(KEY) or {}
    s.window = win
    g_settings.setNode(KEY, s)
    g_settings.save()
end

local megaButton
local megaWindow
local activationPanel
local configPanel
local tickEvent
local minimized = false
local lastGeom

local function currentGeom()
    local pos = megaWindow:getPosition()
    local size = megaWindow:getSize()
    local height = minimized and (megaWindow.maximizedHeight or size.height) or size.height
    return {
        x = pos.x,
        y = pos.y,
        width = size.width,
        height = height,
        minimized = minimized,
    }
end

local function saveWindowState(force)
    if not megaWindow then
        return
    end
    local g = currentGeom()
    if not force and lastGeom and
        lastGeom.x == g.x and lastGeom.y == g.y and
        lastGeom.width == g.width and lastGeom.height == g.height and
        lastGeom.minimized == g.minimized then
        return
    end
    lastGeom = g
    MegaBrain.Persist.saveWindow(g)
end

local function applyMinimized(value)
    minimized = value
    local tabs = megaWindow:recursiveGetChildById("tabs")
    local pages = megaWindow:recursiveGetChildById("pages")
    local bottom = megaWindow:recursiveGetChildById("bottomResizeBorder")
    local right = megaWindow:recursiveGetChildById("rightResizeBorder")

    if minimized then
        megaWindow.maximizedHeight = megaWindow:getHeight()
        megaWindow:setHeight(30)
        tabs:hide()
        pages:hide()
        bottom:hide()
        right:hide()
    else
        megaWindow:setHeight(megaWindow.maximizedHeight or 300)
        tabs:show()
        pages:show()
        bottom:show()
        right:show()
    end
end

local function restoreWindowState()
    local w = MegaBrain.Persist.loadWindow()
    if not w then
        return
    end
    megaWindow:breakAnchors()
    if w.x and w.y then
        megaWindow:setPosition({ x = w.x, y = w.y })
    end
    if w.width and w.height then
        megaWindow:setSize({ width = w.width, height = w.height })
    end
    if w.minimized then
        applyMinimized(true)
    end
    lastGeom = currentGeom()
end

local function onTick()
    saveWindowState()

    -- O MegaBrain só atua com o backend ligado (switch "Ativar Megabrain").
    if not MegaBrain.Backend.isRunning() then
        return
    end

    local payload = MegaBrain.Collector.collect()
    if not payload then
        return
    end

    payload.stopLimit = MegaBrain.Config.monsterStopLimit
    payload.resumeLimit = MegaBrain.Config.monsterResumeLimit
    payload.resumeDelayMs = MegaBrain.Config.resumeDelayMs
    	payload.routeSpacing = MegaBrain.Config.routeSpacing
    	payload.command = MegaBrain.Navigator.getCommand()
    	payload.mode = MegaBrain.Navigator.getMode()

    MegaBrain.Logger.debugLog("Enviando " .. #payload.tiles .. " tiles para o servidor...")
    MegaBrain.Sender.send(payload, function(data)
        MegaBrain.Navigator.onState(data)
        MegaBrain.Navigator.step()
    end)
end

local function toggle()
    if megaWindow:isVisible() then
        saveWindowState(true)
        megaWindow:hide()
        megaButton:setOn(false)
    else
        megaWindow:show()
        megaWindow:raise()
        megaButton:setOn(true)
    end
end

local function closeWindow()
    saveWindowState(true)
    megaWindow:hide()
    megaButton:setOn(false)
end

local function toggleMinimize()
    applyMinimized(not minimized)
    saveWindowState(true)
end

local function bindSwitch(panel, id, onToggle)
    local widget = panel:getChildById(id)
    widget.onClick = function(w)
        w:setOn(not w:isOn())
        onToggle(w:isOn())
    end
    widget:setOn(false)
end

local function initLimit(panel, id, key)
    panel:getChildById(id):setText(tostring(MegaBrain.Config[key]))
end

local function readLimit(panel, id, key)
    local widget = panel:getChildById(id)
    local v = tonumber(widget:getText())
    if v and v >= 0 then
        MegaBrain.Config[key] = math.floor(v)
    else
        widget:setText(tostring(MegaBrain.Config[key]))
    end
end

local function applyConfig()
    readLimit(configPanel, "stopLimit", "monsterStopLimit")
    readLimit(configPanel, "resumeLimit", "monsterResumeLimit")
    readLimit(configPanel, "resumeDelay", "resumeDelayMs")
    readLimit(configPanel, "routeSpacing", "routeSpacing")
    MegaBrain.Persist.save()
end

local function setupTabs()
    local tabs = megaWindow:recursiveGetChildById("tabs")
    local pages = megaWindow:recursiveGetChildById("pages")
    tabs:setContentWidget(pages)

    activationPanel = g_ui.createWidget("MegaBrainActivation")
    tabs:addTab("Ativacao", activationPanel)

    configPanel = g_ui.createWidget("MegaBrainConfig")
    tabs:addTab("Config", configPanel)
end

local function bindUI()
    bindSwitch(activationPanel, "debugSwitch", function(on)
        MegaBrain.Logger.setDebug(on)
    end)
    bindSwitch(activationPanel, "exploreSwitch", function(on)
        MegaBrain.Navigator.setExploring(on)
    end)

    local function setSwitch(id, on)
        local sw = activationPanel:getChildById(id)
        if sw then
            sw:setOn(on)
        end
    end

    -- Rota só é válida em cave totalmente explorada: cai para Explorar e desliga Rota.
    local function fallbackToExplore()
        setSwitch("routeSwitch", false)
        MegaBrain.Navigator.setRouteMode(false)
        setSwitch("exploreSwitch", true)
        MegaBrain.Navigator.setExploring(true)
    end

    bindSwitch(activationPanel, "routeSwitch", function(on)
        if not on then
            MegaBrain.Navigator.setRouteMode(false)
            return
        end
        -- Antes de ligar a Rota, confirma com o backend que não há fronteira a explorar.
        HTTP.getJSON(MegaBrain.Config.commandUrl, function(data, err)
            if err or not data then
                -- Backend indisponível: não dá para validar a exploração.
                setSwitch("routeSwitch", false)
                return
            end
            if data.done then
                MegaBrain.Navigator.setRouteMode(true)
            else
                fallbackToExplore()
            end
        end)
    end)

    -- Exploração concluída: desliga Explorar e liga Rota (não ficar parado).
    MegaBrain.Navigator.onExploreDone = function()
        local ex = activationPanel:getChildById("exploreSwitch")
        if ex and ex:isOn() then
            ex:setOn(false)
            MegaBrain.Navigator.setExploring(false)
        end
        local rt = activationPanel:getChildById("routeSwitch")
        if rt and not rt:isOn() then
            rt:setOn(true)
            MegaBrain.Navigator.setRouteMode(true)
        end
    end

    bindSwitch(activationPanel, "backendSwitch", function(on)
        if on then
            MegaBrain.Backend.start()
        else
            MegaBrain.Backend.stop()
        end
    end)

    activationPanel:getChildById("openInterface").onClick = function()
        local url = MegaBrain.Backend.getUrl()
        if url then
            g_platform.openUrl(url)
        else
            MegaBrain.Logger.debugLog("Backend desligado; nada para abrir.")
        end
    end

    initLimit(configPanel, "stopLimit", "monsterStopLimit")
    initLimit(configPanel, "resumeLimit", "monsterResumeLimit")
    initLimit(configPanel, "resumeDelay", "resumeDelayMs")
    initLimit(configPanel, "routeSpacing", "routeSpacing")
    configPanel:getChildById("applyConfig").onClick = applyConfig

    bindSwitch(configPanel, "autoRouteSwitch", function(on)
        MegaBrain.Config.autoRoute = on
        MegaBrain.Persist.save()
    end)
    configPanel:getChildById("autoRouteSwitch"):setOn(MegaBrain.Config.autoRoute)

    megaWindow:recursiveGetChildById("minimizeButton").onClick = toggleMinimize
    megaWindow:recursiveGetChildById("closeButton").onClick = closeWindow
end

function init()
    MegaBrain.Persist.load()

    megaWindow = g_ui.loadUI("megabrain", g_ui.getRootWidget())
    if not megaWindow then
        return
    end
    megaWindow:hide()

    setupTabs()
    bindUI()
    restoreWindowState()

    megaButton = modules.game_mainpanel.addToggleButton(
        "megabrainButton", tr("MegaBrain"), "/images/options/bot", toggle, false, 99999)
    megaButton:setOn(false)

    tickEvent = cycleEvent(onTick, MegaBrain.Config.pollMs)
end

function terminate()
    saveWindowState(true)

    if tickEvent then
        removeEvent(tickEvent)
        tickEvent = nil
    end
    if megaButton then
        megaButton:destroy()
        megaButton = nil
    end
    if megaWindow then
        megaWindow:destroy()
        megaWindow = nil
    end
end

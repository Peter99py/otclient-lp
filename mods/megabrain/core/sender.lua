MegaBrain.Sender = {}

function MegaBrain.Sender.send(payload, onResponse)
    HTTP.postJSON(MegaBrain.Config.serverUrl, payload, function(data, err)
        if err then
            MegaBrain.Logger.debugLog("Erro HTTP: " .. tostring(err))
        else
            MegaBrain.Logger.debugLog("Resposta do servidor: " .. json.encode(data))
            if onResponse then
                onResponse(data)
            end
        end
    end)
end

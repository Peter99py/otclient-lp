
---

## 11. Loot x Drop: listas unificadas no TargetBot

A lista de itens para descartar fica na aba **Target**, no painel de looting ([looting.otui](targetbot/looting.otui) / [looting.lua](targetbot/looting.lua)). Ela não fica mais na aba Tools.

### 11.1 Interface

| Elemento | Id | Função |
| :--- | :--- | :--- |
| Items to loot | `items` | Itens que serão pegos dos corpos. |
| Loot every item | `everyItem` | Pega todos os itens, **exceto** os que estão em *Items to drop*. |
| Items to drop | `dropItems` | Itens que serão jogados no chão (vindos das bags do jogador). |
| Drop items | `dropEnabled` | Liga/desliga o descarte (padrão: ligado). |

### 11.2 Regras

1. **Exclusão mútua:** se o usuário coloca um item em *Items to drop*, ele é removido de *Items to loot*, e o inverso também (`onItemsUpdate` / `onDropUpdate`). A remoção usa `setItems()`, que não dispara o callback do `UI.Container`, então não há loop.
2. **Loot every item respeita o drop:** em `lootContainer()` a condição é
   `itemsById[id] or (everyItem and not item:isContainer() and not dropById[id])`.
3. **Conflito em perfis existentes:** ao carregar um perfil (`Looting.update`) com o mesmo item nas duas listas, **a lista de loot prevalece** e o item é retirado do drop. A correção vai para o arquivo no próximo save.
4. **Padrão:** perfis sem o campo `drop` começam com os vials `283, 284, 285`. Se o usuário esvaziar a lista, ela continua vazia.
5. **Só com o TargetBot ligado:** `TargetBot.Looting.processDrop()` é chamado pelo loop do `targetbotMacro` (`target.lua`) apenas quando **não há alvo, nem looting**, e o TargetBot não esteve ativo nos últimos 300 ms.
6. **Intervalo:** uma checagem a cada **5000 ms**, movendo um item por vez para a posição do jogador. Containers de corpos (`lootContainer`) são ignorados.

### 11.3 Persistência (perfil do TargetBot)

```json
"looting": {
  "items": [ {"id":3031,"count":1} ],
  "drop": [ {"id":283,"count":1}, {"id":284,"count":1}, {"id":285,"count":1} ],
  "dropEnabled": true,
  "everyItem": true,
  "containers": [ ... ],
  "maxDanger": 10,
  "minCapacity": 100
}
```

> [!NOTE]
> O antigo `storage.dropItems` (aba Tools) não é mais usado nem migrado.

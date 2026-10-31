# Meals

The family picker has a **Meals** button next to **Wall display**. It opens
a Meals screen in the House style with four tabs:

- **Tonight**: today's dinner, or whatever else today has, with **See the
  recipe** when it was planned from one.
- **Week**: today and the next six days. A meal planned from a recipe opens it.
- **Recipes**: the household's Mealie recipes, A to Z, with a search box.
- **Shopping**: the household's shopping list, with a check on each item.

A recipe opens on its own page (`/meals/recipes/<slug>`) in OpenChore's
style: photo, times, ingredients to tick off as they go in, numbered steps
and notes. Nothing ticked there is saved. The recipe page doesn't idle out
to the wall display, so it stays up while someone cooks. **Meals** goes back
to the tab it was opened from; **Who's here?** goes back to the picker.

Recipes are still added and edited in Mealie itself. A signed-in grown-up
sees **Add or edit recipes in Mealie** under the Recipes tab, which opens
Mealie in a new tab.

The data comes from [Mealie](https://mealie.io), a self-hosted recipe
manager. OpenChore talks to Mealie from the server, so the tablet never
holds Mealie's token. Meals and chores are separate: cooking earns no points.

## Who can do what

| | Signed out (shared tablet) | Signed in as a grown-up |
| --- | --- | --- |
| See tonight, the week, recipes and the shopping list | Yes | Yes |
| Tick items off the shopping list | Yes | Yes |
| Add or remove meals on the plan | No | Yes |

To change the plan, open **Week** and tap **Sign in**. After a grown-up signs
in, the app returns to Meals with an **Add a meal** form and a **Remove**
button on each meal. **Who's here?** signs a shared-device session out again.
The server enforces this: changing the plan requires an admin.

## Setting it up

`docker compose up -d` starts Mealie next to OpenChore, on port 9925 by
default. Mealie can't run under a sub-path such as `/meals`, so it gets its
own address. Its colours are set to OpenChore's House theme.

1. Open `http://<your server>:9925` and finish Mealie's first-run setup.
2. In Mealie, open your profile, then **API Tokens**, and create a token.
3. Add it to `.env` next to `compose.yaml`:

   ```bash
   MEALIE_API_TOKEN=<the token>
   # Where browsers reach Mealie, if not http://localhost:9925:
   MEALIE_PUBLIC_URL=http://kitchen.local:9925
   ```

4. Run `docker compose up -d` again. The **Meals** button appears on the picker.

The token acts as the Mealie user who created it, so the plan and shopping
list are that user's household's. The Shopping tab shows the household's
first shopping list.

| Variable | Default | Purpose |
| --- | --- | --- |
| `MEALIE_URL` | `http://mealie:9000` in compose | Where the OpenChore server reaches Mealie |
| `MEALIE_API_TOKEN` | — | A Mealie API token. Meals is off without it |
| `MEALIE_PUBLIC_URL` | `http://localhost:9925` in compose | Where browsers reach Mealie, for **Add or edit recipes in Mealie** and Mealie's own `BASE_URL` |
| `MEALIE_PORT` | `9925` | Host port for the Mealie container |

Already running Mealie elsewhere? Set `MEALIE_URL` in `.env` to your
server and remove the `mealie` service from `compose.yaml`.

## API

| Route | Who | What |
| --- | --- | --- |
| `GET /api/meals/status` | anyone | `{enabled, mealie_url}` |
| `GET /api/meals/plan?start=&end=` | anyone | Meals between two dates (YYYY-MM-DD, at most 31 days) |
| `GET /api/meals/shopping` | anyone | The first shopping list and its items, or `null` |
| `PUT /api/meals/shopping/items/{id}` | anyone | `{checked}` ticks or unticks an item |
| `GET /api/meals/cookbook?search=&page=` | anyone | A page of 24 recipe cards, A to Z, and the `total` |
| `GET /api/meals/cookbook/{slug}` | anyone | One recipe: times, ingredients, steps and notes |
| `GET /api/meals/recipe-images/{id}/{file}` | anyone | A recipe photo, passed on from Mealie |
| `POST /api/meals/plan` | admin | `{date, meal, recipe_id}` or `{date, meal, title}` |
| `DELETE /api/meals/plan/{id}` | admin | Removes a meal |
| `GET /api/meals/recipes?search=` | admin | Up to 20 Mealie recipes by name |

The client in `internal/mealie` is written against Mealie v3.29, the version
`compose.yaml` pins. Check Mealie's release notes before moving the pin.

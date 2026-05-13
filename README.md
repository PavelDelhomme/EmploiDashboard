# Rennes — dashboard événements emploi

Application **Node.js + Express** : carte interactive (Leaflet), liste filtrable, polling des nouveaux événements et **notifications email**. Données ciblées **Rennes et périphérie** (rayon configurable).

## Confusion fréquente : « France Travail Connect » ≠ événements publics

- **[Dispositif API « France Travail Connect »](https://www.data.gouv.fr/dataservices/dispositif-api-france-travail-connect)** : accès **restreint**, flux **OAuth utilisateur** pour récupérer les **données personnelles** d’une personne connectée avec son compte francetravail.fr. Ce n’est **pas** ce qu’il faut pour lister des salons / forums publics.
- **[API « Mes évènements emploi »](https://www.data.gouv.fr/fr/dataservices/api-evenements-france-travail/)** : base d’événements (forums, job datings, etc.) avec **OAuth2 « client credentials »** (identifiant / secret d’application, sans login utilisateur). C’est ce que ce projet utilise.

Inscription et doc produits : [francetravail.io](https://francetravail.io/inscription) — crée une application, souscris à l’API **Mes évènements emploi**, récupère **Client ID**, **Client Secret** et le **scope** exact. La fiche data.gouv : [API Mes évènements emploi](https://www.data.gouv.fr/fr/dataservices/api-evenements-france-travail/). Le portail grand public qui présente le même type d’événements (salons, job datings, ateliers, etc.) est [Mes événements emploi](https://mesevenementsemploi.francetravail.fr/mes-evenements-emploi/) : ce n’est pas une API à scraper, mais la vitrine « humaine » du dispositif ; l’app consomme l’**API partenaire** et retombe sur le portail pour les liens si l’API ne renvoie pas d’URL directe.

Si une requête renvoie **404**, aligne **`FT_EVENTS_PATH`** et les paramètres de requête sur l’**OpenAPI** fournie dans ton espace développeur (le catalogue varie selon les versions de produit). Tu peux activer **`FT_USE_RANGE_HEADER=true`** si la doc indique une pagination par en-tête `Range`.

## Makefile (recommandé)

```bash
make help          # liste des commandes
make env           # crée .env depuis .env.example si besoin
make up            # build + démarrage Docker (voir ports ci-dessous)
make url           # affiche l’URL si le conteneur tourne
make down / logs / ps / shell / build / clean
```

### Ports adaptés sur ta machine

Dans `.env` (voir `.env.example`) :

- **`AUTO_HOST_PORT=true`** (défaut) : au `make up`, un **port libre** est choisi entre `HOST_PORT_RANGE_START` et `HOST_PORT_RANGE_END` ; si `HOST_PORT` est déjà libre, il est conservé.
- **`AUTO_HOST_PORT=false`** : Docker utilise toujours **`HOST_PORT`** (à toi d’éviter les conflits avec d’autres services).

Si ton environnement ne supporte pas la détection via bash `/dev/tcp` (rare), mets `AUTO_HOST_PORT=false` et choisis un port libre manuellement.

## Lancer avec Docker (sans Makefile)

```bash
cp .env.example .env
docker compose up -d --build
```

- Interface : `http://localhost:${HOST_PORT}` (variable d’environnement au moment du `docker compose` ; avec **`make up`**, le script exporte un `HOST_PORT` adapté si besoin).
- **Nom du conteneur** : `rennes-emploi-dashboard` (recherche facile dans `docker ps`).
- **Projet Compose** : `rennes-emploi` (préfixe réseau / ressources).
- Volume SQLite nommé : `rennes-emploi-sqlite`.

### Mode démo sans API France Travail

```env
MOCK_FT=true
```

Tu peux quand même tester SMTP et l’interface.

## Développement local (sans Docker)

```bash
make install   # ou : npm install
make dev       # ou : npm run dev
```

## Git : branches de travail

Modèle simple :

| Branche | Rôle |
|---------|------|
| `main` | version stable / livrable |
| `develop` | intégration des merges |
| `feature/*` | développement (ex. `feature/tooling-makefile-ports`) |

Les branches peuvent coexister sur le même commit ; à toi d’ouvrir des **pull requests** `feature/*` → `develop`, puis `develop` → `main` sur GitHub.

Pour pousser : configure `git remote add origin git@github.com:USER/REPO.git` (ou `git remote set-url`), puis `git push -u origin main` (et les autres branches si besoin). Tu peux aussi utiliser `make push` si ton `origin` est déjà défini.

## Variables utiles

| Variable        | Rôle |
|----------------|------|
| `HOST_PORT` / `AUTO_HOST_PORT` / `HOST_PORT_RANGE_*` | Port publié sur ta machine ; auto si `AUTO_HOST_PORT=true` |
| `FT_CLIENT_ID` / `FT_CLIENT_SECRET` | OAuth2 client credentials |
| `FT_OAUTH_SCOPE` | Scope **exact** fourni par le portail pour ton app |
| `FT_EVENTS_PATH` | Chemin de la ressource (à aligner sur la doc FT si besoin) |
| `FT_FILTER_CODE_POSTAL` / `FT_FILTER_DEPARTEMENT` | Filtres géo envoyés à l’API (si prévus par le produit) |
| `FT_PORTAL_BASE_URL` / `FT_PORTAL_EVENT_URL_TEMPLATE` | Portail [Mes événements emploi](https://mesevenementsemploi.francetravail.fr/mes-evenements-emploi/) et modèle de lien détail |
| `FT_USE_RANGE_HEADER` / `FT_RANGE_PAGE_SIZE` | Pagination type `Range` (si doc produit) |
| `RENNES_LAT` / `RENNES_LON` / `DEFAULT_RADIUS_KM` | Zone géographique |
| `SMTP_*` / `MAIL_FROM` | Envoi des alertes |
| `FT_SEED_SILENT` | Premier cycle : enregistre les événements existants **sans** envoyer d’emails |

## Licence

Projet modèle : adapte la licence selon ton usage.

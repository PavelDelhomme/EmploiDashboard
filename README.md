# Rennes — dashboard événements emploi

Application **Node.js + Express** : carte interactive (Leaflet), liste filtrable, polling des nouveaux événements et **notifications email**. Données ciblées **Rennes et périphérie** (rayon configurable).

## Confusion fréquente : « France Travail Connect » ≠ événements publics

- **[Dispositif API « France Travail Connect »](https://www.data.gouv.fr/dataservices/dispositif-api-france-travail-connect)** : accès **restreint**, flux **OAuth utilisateur** pour récupérer les **données personnelles** d’une personne connectée avec son compte francetravail.fr. Ce n’est **pas** ce qu’il faut pour lister des salons / forums publics.
- **[API « Mes évènements emploi »](https://www.data.gouv.fr/fr/dataservices/api-evenements-france-travail/)** : base d’événements (forums, job datings, etc.) avec **OAuth2 « client credentials »** (identifiant / secret d’application, sans login utilisateur). C’est ce que ce projet utilise.

Inscription et doc produits : [francetravail.io](https://francetravail.io/inscription) — crée une application, souscris à l’API événements, récupère **Client ID**, **Client Secret** et le **scope** exact indiqué pour ton app (copie-le dans `FT_OAUTH_SCOPE`). Si une requête renvoie **404**, vérifie dans la doc catalogue le chemin exact de la ressource et mets à jour `FT_EVENTS_PATH` (et éventuellement `FT_EXTRA_QUERY` dans `.env`).

## Lancer avec Docker (port au choix sur ta machine)

```bash
cp .env.example .env
# Édite .env : HOST_PORT (ex. 3080), identifiants FT, SMTP…
docker compose up -d --build
```

- Interface : `http://localhost:${HOST_PORT}` (défaut **3080**, donc pas de conflit avec un autre service sur **3000**).
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
npm install
cp .env.example .env
npm run dev
```

## Créer le dépôt GitHub

```bash
cd /chemin/vers/EmploiDashboard
git init
git add .
git commit -m "Initial import : dashboard événements emploi Rennes"
gh repo create rennes-emploi-dashboard --private --source=. --push
```

(Sans GitHub CLI : crée un dépôt vide sur GitHub puis `git remote add origin …` et `git push -u origin main`.)

## Variables utiles

| Variable        | Rôle |
|----------------|------|
| `HOST_PORT`    | Port sur l’hôte mappé vers le port 3000 du conteneur |
| `FT_CLIENT_ID` / `FT_CLIENT_SECRET` | OAuth2 client credentials |
| `FT_OAUTH_SCOPE` | Scope **exact** fourni par le portail pour ton app |
| `FT_EVENTS_PATH` | Chemin de la ressource (à aligner sur la doc FT si besoin) |
| `RENNES_LAT` / `RENNES_LON` / `DEFAULT_RADIUS_KM` | Zone géographique |
| `SMTP_*` / `MAIL_FROM` | Envoi des alertes |
| `FT_SEED_SILENT` | Premier cycle : enregistre les événements existants **sans** envoyer d’emails |

## Licence

Projet modèle : adapte la licence selon ton usage.

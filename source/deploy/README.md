# Plateforme en ligne

Le même programme sert aux deux usages :

- **hors ligne / salle de jeu** : double-clic sur `pairing_easy.exe` ; bouton « Ouvrir au réseau local » (onglet Inscriptions) pour les tablettes, écrans et inscriptions sur le Wi-Fi de la salle ;
- **plateforme en ligne** : serveur avec HTTPS, comptes et droits.

## Installation en ligne (Docker)

```bash
cd deploy
DOMAIN=appariements.mon-club.fr docker compose up -d --build
```

Caddy obtient le certificat HTTPS. Les données sont dans le volume `data` (base SQLite, base FFE, journaux) : sauvegardez-le.

Sans Docker : `pairing_easy -listen :8080 -public-url https://appariements.mon-club.fr -trust-proxy` derrière un proxy HTTPS (nginx, Caddy…), en transmettant `X-Forwarded-For` et `X-Forwarded-Proto` et sans mise en tampon pour `/api/*/stream`.

## Comptes et droits

| Rôle | Peut |
|---|---|
| Administrateur | tout : comptes, système, tous les événements |
| Gestionnaire | créer des événements et les gérer, donner des droits sur ses événements |
| Utilisateur | accéder aux seuls événements partagés avec lui |

Droits par événement : propriétaire, arbitre, **saisie des résultats** (lecture seule ; saisit en un clic les résultats encore vides de la ronde en cours, sans pouvoir corriger), lecture seule.

## Passer d'en ligne à hors ligne

Réglages de l'événement → « Exporter (fichier) » sur la plateforme, « Importer un fichier » sur l'ordinateur de la salle. Après le tournoi, exportez depuis la salle puis « Remplacer depuis un fichier » sur la plateforme.

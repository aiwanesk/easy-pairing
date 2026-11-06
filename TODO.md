# pairing_easy — reste à faire

Mis à jour le 6 novembre 2026.

## À faire

- [ ] **Export Papi pour l'homologation FFE.** Produire un fichier .papi que Papi et le site fédéral acceptent. Bloqué : il faut un vrai fichier .papi d'un ancien tournoi pour en relever la structure exacte (bases Access).
- [ ] **Coupe par équipes** (Coupe de France, Loubatière…). La coupe individuelle et les équipes en suisse ou en toutes-rondes existent ; il manque la combinaison : tableau d'équipes, départage d'un match nul (échiquiers, Armageddon…).
- [ ] **Valider la plateforme en ligne** sur un vrai serveur : `source/deploy` (Docker + Caddy, HTTPS automatique). Écrit mais pas lancé faute de Docker dans l'environnement de développement.
- [ ] **Grille américaine** : format FFE en place (+12B, =5N, -3B, +F/-F, EXE), ex aequo, départages, choix de la ronde. À confirmer par Alex.

## Améliorations possibles

- [ ] Normes : compter les points au-delà du minimum comme parties supplémentaires, ignorer des parties gagnées, exemption de fédérations des opens (FIDE B.01 1.4.1 et 1.4.3 d).
- [ ] Coupe : prix et normes (aujourd'hui masqués pour les coupes).
- [ ] Équipes : prix par équipe, classement individuel par échiquier, ordre de force imposé (règle FFE des Interclubs).
- [ ] Export TRF et PGN pour les coupes et les tournois par équipes.
- [ ] Synchronisation automatique entre la plateforme en ligne et une installation hors ligne (aujourd'hui : export puis import d'un fichier).

## Fait le 6 novembre

Lien d'inscription (liste de contrôle et boutons), accès réseau local activable depuis l'interface ; prix Hort, partage égal et conditions (catégories, Elo, sexe, fédérations, club, non classés, score) ; départages et prix choisis à la création ; Elo rechargés quand l'Elo utilisé change avant la ronde 1 ; nombre de rondes affiché en direct ; exempts en un clic ; explication détaillée d'un appariement impossible ; normes FIDE (B.01 2024) et joueurs proches d'une norme ; tables de Varma ; coupe individuelle ; équipes (suisse et toutes-rondes) ; rôle Gestionnaire et droit Saisie des résultats ; mode plateforme en ligne et export/import d'événement ; remplacement automatique de l'ancienne version au lancement ; charte graphique (voir source/CHARTE-GRAPHIQUE.md).

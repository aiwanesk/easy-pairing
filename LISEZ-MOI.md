# pairing_easy

Gestion de tournois d'échecs : système suisse (hollandais FIDE, Burstein, accélération Baku) par bbpPairings, toutes-rondes (tables de Berger), comptes et rôles, journal d'audit, pointage, inscriptions en ligne, écrans et kiosques. Un seul exécutable, données dans une base SQLite.

## Lancer (Windows)

Double-cliquez sur **`pairing_easy.exe`**. Au premier lancement :

1. le programme télécharge la dernière version officielle de bbpPairings depuis GitHub ; sans Internet, il installe la copie intégrée à l'exécutable (compilée depuis les sources officielles) ;
2. il démarre le serveur et ouvre le navigateur sur http://localhost:8080 ;
3. la page « Première installation » crée le compte administrateur.

Fermer la fenêtre noire arrête le programme. Relancer l'exe alors qu'il tourne déjà rouvre simplement le navigateur.

Données : `%LOCALAPPDATA%\pairing_easy` (base `pairing_easy.db`, moteur, base FFE, journaux techniques). Pas dans Documents ni sur le Bureau, que OneDrive peut synchroniser et verrouiller pendant l'écriture. Sauvegardez ce dossier.

Options (raccourci ou invite de commandes) :

| Option | Rôle |
|---|---|
| `-lan` | écoute sur le réseau local : tablettes de pointage, écrans, inscriptions depuis d'autres appareils (les adresses s'affichent au démarrage) |
| `-port 8080` | port HTTP |
| `-data <dossier>` | autre emplacement des données |
| `-bbp <fichier>` | utiliser un bbpPairings précis |
| `-no-browser` | ne pas ouvrir le navigateur |
| `-log-level debug` | journalisation détaillée |

macOS et Linux : binaires dans `autres-systemes/`. bbpPairings n'est intégré qu'à la version Windows : sur Mac, `Lancer pairing_easy (Mac).command` le compile une fois (outils Xcode requis) puis lance l'application.

## Fonctions

**Comptes et sécurité.** Administrateurs et utilisateurs ; droits par événement : propriétaire, arbitre, lecture seule. Mots de passe PBKDF2-SHA256 (600 000 itérations), sessions en cookie HttpOnly, limitation des tentatives de connexion, protection CSRF.

**Journal d'audit.** Chaque action est enregistrée avec son auteur, son adresse IP et le détail avant/après : connexions et échecs, comptes, réglages, joueurs, pointage, inscriptions, appariements, résultats, kiosques. Recherche et filtres. Les journaux techniques (JSON, un fichier par jour, 30 jours) sont dans `logs/`.

**Événements et tournois.** Plusieurs tournois par événement. Suisse hollandais ou Burstein (bbpPairings), accélération Baku (FIDE C.04.5), barème configurable, valeur de l'exempt, couleur initiale. Toutes-rondes simple ou double, tables de Berger FIDE, inversion des deux dernières rondes du premier cycle, numéros par Elo ou tirage au sort.

**Mode FIDE.** Refuse les actions interdites par les règlements : une modification manuelle différente du moteur est annulée, seule la ronde précédente peut être corrigée, le barème et le système sont figés, les appariements interdits sont fixés après la ronde 1.

**Joueurs.** Ajout depuis la base FFE (Data.mdb de Papi, lue en Go pur), par code FIDE (fiche ratings.fide.com), à la main, ou en masse (licences, codes FIDE ou lignes CSV copiées d'un tableur). Byes ½ / 1 / 0 par ronde, joueurs sans droit aux byes, retraits, appariements interdits (familles, clubs), droits d'inscription et paiements.

**Inscriptions en ligne.** Formulaire public par tournoi, préremplissage par la base FFE, places, date limite, fourchette Elo, validation par l'arbitre ou automatique, liste publique des inscrits, anti-spam.

**Pointage.** Seuls les joueurs pointés sont appariés à la ronde 1 (suisse) ou entrent au calendrier (toutes-rondes). Pointage par l'arbitre ou en libre-service sur une tablette (kiosque protégé par code).

**Rondes.** Appariement, puis publication : les écrans et pages publiques restent sur la ronde précédente tant que l'arbitre n'a pas vérifié. Saisie des résultats (forfaits compris) par l'arbitre, ou par les joueurs au kiosque sans écraser un résultat existant. Modification manuelle avec avertissements (rencontres répétées, appariements interdits, couleurs, exempt déjà reçu) et comparaison avec bbpPairings. Journal du tournoi repris dans le TRF.

**Classements.** Départages FIDE C.07 configurables et ordonnables : Buchholz, Buchholz tronqué -1 et -2, Buchholz médian, Sonneborn-Berger, cumulatif, confrontation directe, victoires, parties gagnées, parties avec les Noirs, victoires avec les Noirs, ARO, performance, Koya. Grille américaine et grille de Berger. Prix avec filtres (catégorie, Elo maximum, féminines, club), sans cumul, avec la liste des ex aequo sans prix.

**Public et écrans.** Pages publiques en direct (appariements par échiquier ou par nom, joueurs, classement, grille). Mode écran pour vidéoprojecteur avec rotation des vues, défilement automatique et bandeau d'information.

**Exports.** TRF (FIDE), PGN de toutes les rondes, CSV des joueurs et du classement, impression de chaque vue.

## Par rapport à Sharly Chess

Tickets ouverts de Sharly Chess traités :

| Ticket | État |
|---|---|
| #2460 Écran grille de Berger | fait : vue « Grille » du mode écran |
| #2374 Pointage pour les toutes-rondes et les tournois à élimination directe | fait pour les toutes-rondes ; l'élimination directe n'est pas gérée |
| #2334 Ajout de joueurs en masse | fait |
| #1061 Prix : lister les ex aequo sans prix | fait |
| #2280 Paquet AppImage pour Linux | remplacé par un binaire Linux autonome |
| #1776 Extension Chess-Results | non traité |

Fonctions de Sharly absentes à ce jour : tournois par équipes, Molter/Scheveningen, élimination directe, calcul des normes, publication vers Chess-Results et la FFE, export Papi, PGN avec les coups, minuteurs, tournois sur plusieurs périodes, extensions.

## Développement

```bash
cd frontend && npm install && npm run dev      # Vite sur :5173 (proxy /api vers :8080)
cd backend && go run ./cmd/pairing_easy -bbp /chemin/bbpPairings -no-browser
build/build.sh                                  # dist/ : Windows, macOS (arm64 et Intel), Linux
build/build-bbp-windows.sh                      # recompile le bbpPairings intégré (MinGW)
```

Go 1.27 et React 19. Le backend n'a qu'une dépendance (SQLite en Go pur, sans cgo), vendorisée : la compilation fonctionne hors ligne.

Tests : `cd backend && BBP_PAIRINGS=/chemin/bbpPairings go test ./...`. Ils couvrent :

- trois opens complets de 9 rondes (hollandais, Burstein, Baku) avec byes, forfaits, retrait, inscription tardive, inscription en attente, pointage et appariements interdits, intégralement revérifiés par `bbpPairings -c` ;
- les tables de Berger, les toutes-rondes doubles, la confrontation directe, les prix et l'appariement manuel ;
- un scénario API complet : installation, comptes, droits, CSRF, inscription publique, kiosque, rondes, publication, exports, journal d'audit, déconnexion.

```
backend/internal/
  chess/   modèle, suisse (TRF, Baku, interdits), Berger, départages, prix, appariement manuel
  bbp/     exécution de bbpPairings, installation (release GitHub ou copie intégrée)
  store/   SQLite : comptes, sessions, droits, événements, audit
  auth/    PBKDF2, jetons de session
  api/     API admin et publique, SSE, kiosque, exports
  lookup/  base FFE (Papi) et fiches FIDE
  mdb/     lecteur Access Jet4 en Go pur
frontend/src/  admin/ (événements, tournois, comptes, audit, système) et public/ (pages, inscription, kiosque, écran)
```

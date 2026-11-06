# pairing_easy sous Windows

Deux façons de le faire tourner sur votre ordinateur.

## 1. Utiliser l'application (sans rien installer)

Double-cliquez sur `pairing_easy.exe`. Le navigateur s'ouvre sur http://localhost:8080 ; au premier lancement, la page « Première installation » crée le compte administrateur. Fermer la fenêtre noire arrête le programme.

Les données sont dans `%LOCALAPPDATA%\pairing_easy` (base, moteur bbpPairings, base FFE, journaux). Sauvegardez ce dossier.

Pour les tablettes, écrans et inscriptions sur le Wi-Fi de la salle : onglet Inscriptions d'un tournoi → « Ouvrir au réseau local », ou lancement avec `-lan`. Windows demande alors d'autoriser le programme dans le pare-feu : acceptez pour les réseaux privés.

Une nouvelle version de `pairing_easy.exe` arrête d'elle-même l'ancienne au lancement : remplacez le fichier et double-cliquez.

## 2. Compiler et lancer depuis les sources

### Prérequis

- Go 1.27 ou plus : `winget install GoLang.Go`
- Node.js 20 ou plus : `winget install OpenJS.NodeJS.LTS`

Rouvrez l'invite de commandes après l'installation pour que `go` et `node` soient trouvés. Aucune autre dépendance : le serveur n'utilise que des modules Go fournis dans `backend\vendor`, et le moteur bbpPairings pour Windows est intégré à l'exécutable.

### Lancer

Double-cliquez sur **`build-and-run.bat`** (à côté de `pairing_easy.exe`) : il décompresse `source.zip` la première fois, puis appelle `source\start-windows.bat`. Le script vérifie Go et Node, compile l'interface puis le serveur dans `dist\pairing_easy.exe` et le lance. La première fois, `npm ci` télécharge les paquets de l'interface (connexion Internet nécessaire, une minute environ).

Depuis une invite de commandes, dans le dossier `source` :

| Commande | Effet |
|---|---|
| `start-windows.bat` | compile et lance (mêmes données que l'application) |
| `start-windows.bat run -lan` | idem, ouvert au réseau local ; toute option de pairing_easy peut suivre |
| `start-windows.bat dev` | développement : serveur Go sur :8080 et interface Vite sur http://localhost:5173 avec rechargement à chaud ; données séparées dans `source\.data-dev` |
| `start-windows.bat build` | compile seulement `dist\pairing_easy.exe` |
| `start-windows.bat test` | lance les tests Go |

En PowerShell, l'équivalent direct est `.\build\dev-windows.ps1 [run|dev|build|test]`.

### Options de pairing_easy

| Option | Rôle |
|---|---|
| `-lan` | écoute aussi sur le réseau local |
| `-port 8080` | port HTTP |
| `-data <dossier>` | autre dossier de données |
| `-bbp <fichier>` | utiliser un bbpPairings précis |
| `-no-browser` | ne pas ouvrir le navigateur |
| `-log-level debug` | journalisation détaillée |
| `-listen :8080 -public-url https://… -trust-proxy` | plateforme en ligne derrière un proxy HTTPS (voir `deploy\README.md`) |

### En cas de problème

« Go n'est pas installé » ou « Node.js n'est pas installé » : installez-le avec la commande indiquée, fermez puis rouvrez la fenêtre.

« le port 8080 est occupé » : une autre application utilise ce port ; lancez avec `-port 8081`.

La compilation de l'interface échoue après une mise à jour des sources : supprimez `frontend\node_modules` et relancez.

PowerShell refuse d'exécuter le script : passez par `start-windows.bat`, qui lance PowerShell avec l'autorisation nécessaire pour ce seul script.

# Charte graphique de pairing_easy

L'identité part de la salle de tournoi : le vert des échiquiers de compétition, la case claire, le laiton des pendules. Elle doit rester lisible sur un portable d'arbitre comme sur un vidéoprojecteur.

## Couleurs

| Rôle | Nom | Valeur | Usage |
|---|---|---|---|
| Couleur principale | Vert échiquier | `#1F5C4A` | boutons principaux, onglet actif, sélection, rondes terminées, liens |
| Fond de navigation | Vert profond | `#12302A` | barre latérale |
| Fond | Papier | `#F6F8F6` | fond des pages |
| Surfaces | Blanc | `#FFFFFF` | cartes, tableaux |
| Teinte douce | Case claire | `#EEF3EE` / `#E3EFE9` | survols, sélection douce, motif de la page de connexion |
| Texte | Encre | `#17202A` | texte courant |
| Texte secondaire | Gris vert | `#5F6B66` | légendes, aides |
| Avertissement | Laiton | `#9A6212` sur `#FAF1DF` | ronde en cours, brouillons, titres FIDE (GM, MI, FM…) |
| Erreur, perte | Drapeau | `#B42318` sur `#FDEAE8` | erreurs, suppression, défaites dans la grille |
| Succès, gain | Vert | `#1E6B4E` sur `#E1EFE7` | confirmations, gains dans la grille |

Une seule couleur d'action : le vert échiquier. Le laiton signale ce qui attend une décision, le rouge ce qui est en erreur ou irréversible.

## Élément signature

Le bandeau de chaque événement et de chaque tournoi est un échiquier vert profond (`#12302A`) dont le damier s'estompe vers la gauche. Le nom du tournoi y est composé en grand, et les rondes y sont des cases : claire = terminée, laiton = en cours, sombre = à venir, pointillé = non publiée. C'est le seul endroit où la couleur forte s'affiche en aplat ; la barre latérale reste claire. La page de connexion et l'en-tête public reprennent le même vert profond.

## Typographie

Deux familles, embarquées dans l'application (aucun accès Internet nécessaire) :

- Archivo étroit (variable, chasse 75 à 85 %, graisse 700 à 800), pour les titres, le logo et les chiffres du jeu : numéros d'échiquier, points, rondes, en-têtes de tableaux, comme sur un tableau d'affichage ;
- Public Sans (variable, embarquée dans l'application : aucun accès Internet nécessaire), chiffres tabulaires partout pour que les colonnes de points, d'Elo et de départages s'alignent.

Titre de page 34 px (40 px dans le bandeau) en Archivo étroit extra-gras ; titre de carte 19 px Archivo étroit gras ; sous-titre 13 px demi-gras ; texte 14 px ; aides 12 px. Pas de majuscules d'apparat : les intitulés s'écrivent en minuscules avec une capitale initiale.

## Formes

Bandeau arrondi à 14 px, cartes à 12 px, champs et boutons à 6 px. Les blocs de travail (appariements, classements, grilles) portent un filet vert de 3 px en haut. Dans les appariements, le numéro d'échiquier est une case alternée claire et vert pâle.

Le logo est tiré de l'emblème de la Ligue corse des échecs, avec l'accord de son président : la tête de Maure est retirée, il reste le roi blanc debout et une seule pièce au sol, passée en noir (dessin à l'encre). Fichiers dans `brand/` (`logo-corse-1024.png`, script `generate_logo_corse.py`) ; dans l'application, `logo.png` (connexion) et `logo-32/64/180/192.png` (icônes). Toujours sur fond blanc, coins arrondis quand il est posé sur le vert.

## Ton

Phrases courtes, verbes d'action sur les boutons (« Apparier la ronde 5 », « Ouvrir les inscriptions »), même mot dans le bouton et dans la confirmation. Les erreurs disent ce qui bloque et comment le débloquer.

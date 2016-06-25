# InterPlanetary Game System Architecture

## Introduction

### Overview

This is a proposal for a decentralized turn-based game system based on the IPFS distributed data sharing network. It enables entities on the Internet to play turn based games against each other without a central server. The initial proposal supports the game go (baduk, weiqui) but the game specific details are modular enough to allow any turn based game to be plugged into the system. The aim of this RFC is to lay out the system architecture and act as a guide for implementing the external parts of a functional system.

### IPFS

The [IPFS](https://ipfs.io) project implements a globally distributed content addressed directed acyclic graph blob data storage system built on top of a decentralized trust-less peer-to-peer network. It allows immutable objects to be shared among the peers in the network, and also supports direct peer-to-peer communication through API bindings.

### Go, the game

Go, also known as baduk and weiqui, is an ancient and still popular turn based strategy game. It is played by two players, Black and White, who take turns placing black and white stones, respectively, on the intersections of a grid of lines. The stones never move around the board once they are placed. The stones may be captured and removed from the board under specific conditions. The game proceeds until one of the players resigns or until they both pass their turn consecutively. If the game did not end in resignation, the winner is the player that has claimed the most territory. The specific means of counting territory and the finer points of the endgame vary among scoring systems, but this level of detail is sufficient for the following discussion. More information about the game may be found at [Sensei's Library](http://senseis.xmp.net). 


## IPNS Namespace

All of the public data for a node is stored in the `/ipns/[node-id]/interplanetary-game-system/` namespace. This namespace is a tree signed with the node private key containing all of the objects necessary for the system. The namespace will be abbreviated in this document as `/ipns/[state]/`. 


## IPNS Content Outline

```
ipns/[node-id]/interplanetary-game-system/
| data: "[last-updated-timestamp]"
| identity.asc
| my-nodes.txt
| my-nodes.sig
| version
| challenges/
| | challenge-offer-1-head/ # ipgs-commit
| | | committer-public-key
| | | parent
| | | data
| | challenge-offer-2-head/ # ipgs-commit
| | | committer-public-key
| | | parent
| | | data
| current-games/
| | current-game-1-head/ # ipgs-commit
| | | committer-public-key
| | | parent
| | | data
| archive/
| | player-hash-1/
| | | game-hash-1 # game-history-object
| | | game-hash-2 # game-history-object
| | player-hash-2/
| | | game-hash-3 # game-history-object
| | | game-hash-4 # game-history-object
| players/
| | player-public-key-hash-1 # player-data-object
| | player-public-key-hash-2 # player-data-object
| | player-public-key-hash-3 # player-data-object
```


## Player Identity

Though the IPFS system has the concept of nodes identified by their public/private key pair, this is insufficient for tracking players in a game network. A player may wish to maintain multiple access nodes to the network (home computer, mobile device, backup server). Sharing the node key pair between these machines is unsupported, could pose a security risk, and would probably be inconvenient.

A standard GPG key pair is proposed as the basis of identity for a player entity in this system. The master key pair is the main source of a player's identity, while a signing subkey is used in the day to day interactions with the system.  An existing GPG key could be used, or a new one could be generated for the game system. 

The public half of the key is stored at `/ipns/[state]/identity.asc` as ASCII armored text file. This form of public signature can be created using the `gpg --armor --output identity.asc --export player@ipgs` command. The IPFS hash of this file is referred throughout this document as `[public-key-hash]` (possibly with prefixes such as `[committer-public-key-hash]` or `[player-public-key-hash]`.

The list of nodes, including the current node, associated with the identity is stored at `/ipns/[state]/my-nodes.txt`. The file contains Node-IDs, one per line. The associated GPG signature for this list is stored at `/ipns/[state]/my-nodes.sig`. This kind of signature can be created using the `gpg --output my-nodes.sig --detach-sig my-nodes.txt` command.


## Player Rating

The [Glicko2 Rating System](https://en.wikipedia.org/wiki/Glicko_rating_system) is used in IPGS to rate players on a per-game basis. Each player calculates their own rating based on their knowledge of the games that they have played. They also calculate the ratings of all of the other players that they are aware of, taking into account their level of trust in the sources of the information about those other players. The algorithm for all of this is to be determined.

Ratings are described by objects with the following structure:

```
{
	"R": [rating],
	"RD": [rating-deviation],
}
```

The `[rating]` is the numerical rating. The `[rating-deviation]` is the numerical rating deviation, a measure of the certainty of the rating.


## Version

The version of the protocol implemented by the game node is published in `/ipns/[state]/version`. This link points to the protocol description document in the `/ipfs` namespace.


## Timestamps

All of the timestamps used throughout the system are in the RFC3339 format in the UTC timezone. For example: `2016-01-25T17:45:15.2Z`. Up to 9 decimal places may be used in the seconds field, though it is unlikely that any are actually necessary. The nodes should be using NTP to synchronize their node clocks with the internet for best results.


## Last-Updated

The `/ipns/[state]/last-updated` node contains a simple timestamp in its data field indicating the last time the namespace was updated.

## IPGS Commits

IPGS Commits are used to track the history of an ongoing game from the Game Challenge phase through the full Current Game Record history. These objects have the following structure:

```
{
	"data": {
		"timestamp": "[commit-timestamp]",
		"commit-type": "[commit-type-name]",
		"signature": "[commit-signature]"
	},
	"links": [
		{ "hash": "[committer-public-key-hash]", "size": [committer-public-keys-size],
		  "name": "commiter-public-key" },
		{ "hash": "[parent-commit-hash]", "size": [parent-commit-size],
		  "name": "parent" },
		{ "hash": "[commit-data-hash]", "size": [commit-data-link],
		  "name": "data" }
	]
}
```

The `parent` link is omitted for the first commit.

The `[commit-signature]` is the ASCII-armored signature using the player's private key of the following string: `[commit-timestamp]|[commit-type-name]|[committer-public-key-hash]|[commit-data-hash]|[parent-commit-hash]`.  The `[parent-commit-hash]` is replaced with the string `none` for the first commit. This signature can be created using the `echo [string-to-sign] | gpg --armor --output [tmp-file] --detach-sig -` commands.

The `[commit-type-name]` may be one of the following:

 * `challenge-offer`
 * `challenge-accept`
 * `challenge-confirm`
 * `game-step`

The `data` link points to an appropriate object based on the `[commit-type-name]`.


## Game Challenges

The game challenges list stored in `/ipns/[state]/challenges/` is a flat list containing the first commits of Current Game Records. The challenges in this list are identified by the name `[challenging-player-hash]|[challenge-timestamp]`. Players wishing to accept the challenge may create an analogous entry in their Current Games list and committing their challenge acceptance to the Current Game Record. A game is considered started when the challenger also moves the Current Game Record to their current games list and commits their challenge confirmation to the record.  If multiple contenders accept a public challenge, the choice of opponent is left to the challenger.

### Challenge Offer

The challenge offer is a type of Current Game Record commit. Its data payload contains the following data:

```
{
	"timeout": "[challenge-timeout-timestamp]",
	"game": "[game-name]",
	"rules": "[scoring-rules-hash]",
	"ranked": true,
	"time-control": {
		"type": "[time-control-type]",
		// other required time parameters
	},
	"challenger-rating": {
		"R": 1500,
		"RD": 350
	},
	"target-rating": {
		"R": 1500,
		"RD": 300
	},
	"first-turn": "automatic",
	"comments": "arbitrary text comments",
	"board-width": 19,
	"board-height": 19,
	"komi": 6.5,
	"handicap": 0
}
```

The `timeout` field indicates the date and time when the challenge will expire.

The `game` field currently only supports the value `go`.

The `rules` field points to a game rules description object stored in `/ipfs/[scoring-rules-hash]`.

Only games that have the `ranked` field set to true should be treated as affecting the players' ratings.

The `time-control` object should describe the time control for the game. The required fields depend on the specific `type` selected. The various types of time control structures are described below.

The `challenger-rating` field specifies the challenger's current assessment of their rating.

The `target-rating` field specifies the rating of the players that the challenge is targeting. This is the rating at which the challenger would like to play. Players within this rating range are welcome. Players outside of this range may accept the challenge, but are less likely to get a game confirmation from the challenger.

The `first-turn` field specifies the player that will be making the first move of the game. This field may be set to `challenger`, `contender`, or `automatic`. The `automatic` value indicates that the game rules decide the player who gets the first turn. In go this would usually be the lower-ranked player playing black.

The `comments` field is an arbitrary text data field to be used at the challenger's discretion.

#### Time Control: `absolute`

```
{
	"type": "absolute",
	"seconds-per-player": 36000
}
```

This `time-control` `type` specifies the absolute number of seconds provided to each player. The maximum play time is therefore 2 * `seconds-per-player` seconds.

#### Time Control: `fixed`

```
{
	"type": "fixed",
	"seconds-per-move": 3600
}
```

This `time-control` `type` specifies the fixed number of seconds provided to each player to make one move. The time is reset for each move. The maximum play time is therefore `[total-number-of-moves]` * `seconds-per-move` seconds.

#### Game Specific Fields: `go`

For go, the `board-width` and `board-height` fields specify the shape of the game board. The `komi` field specifies the number of points given to the white (second) player in compensation for giving up the first move. The `handicap` field specifies the number of free stones given to the first player. The rules specify the placement of these handicap stones (fixed points or player choice). The `handicap` field may be set to -1 for automatic handicap calculation based on the two players' ratings.

### Challenge Acceptance

The challenge acceptance is a type of Current Game Record commit. Its data payload contains the following data:

```
{
	"timeout": "[challenge-acceptance-timeout-timestamp]",
	"condender-rating": {
		"R": 1473,
		"RD": 20,
	},
	"comments": "arbitrary text comments",
}
```

The `timeout` field indicates the date and time when the challenge acceptance will expire.

The `contender-rating` field specify the rating that the contender claims to have. The challenger is free to rely on them, use its own calculation of the contender's rating, or some combination of the two.

The `comments` field is an arbitrary text data field used at the contender's discretion.

### Challenge Confirmation

The challenge confirmation is a type of Current Came Record commit. Its data payload contains the following data:

```
{
	"timeout": "[challenge-confirmation-timeout-timestamp",
	"first-turn": "challenger",
	"handicap": 2,
	"comments": "arbitraty test comments"
}
```

The `timeout` field indicates the date and time when the challenge confirmation will expire. The first move must be played before this date and time.

The `first-turn` and `handicap` fields indicate the final values for the same challenge commit fields. These final values are required if the values in the challenge commit indicated automatic selection of the values.

The `comments` field is an arbitrary test data field used at the challenger's discretion.

### Game Cancellation

A Current Game Record that goes through the three stages of challenge, but does not get a single `game-step` commit before the challenge confirmation timeout is deleted from the current games list. If the contender was supposed to have made the move, the challenger is free to mark this event as a black mark in their player database.


## Current Games

The current games list stored in `/ipns/[state]/current-games/` is the primary means of communicating game state between players. The Current Game Record objects in this list are identified by the name `[challenging-player-hash]|[challenge-timestamp]|[accepting-player-hash]`.

### Current Game Record

The Current Game Record is a series of commit objects. The initial commit is the challenge for the game. The second commit is the notification that the game is accepted. The third commit is the confirmation by the challenger that the challenge was accepted. All subsequent commits are individual game steps.

### Game Step

The game step is a type of Current Game Record commit. Its data payload contains the following data:

```
";[SGF-node]"
```

Possible variations of the SGF-nodes that a commit might contain are listed below:

 * `;B[ab]`
 * `;W[de]C[a comment here]`
 * `;C[a comment by itself]`
 * `;B[]` (black passes)
 * `;RE[W+3.5]` after two consecutive passes
 * `;RE[B+Resign]`
 * `;RE[W+Time]`
 * `;RE[B+Forfeit]`

The game ends when both players follow a `;RE[...]` node with a pair of `;C[]` nodes.

## Finished Games

Games that have been resolved but don't have a signature yet are listed in `/ipns/[state]/finished-games/`. When a game is deemed to be finished its Current Game Record objects are converted into a final standardized SGF game record file that will be used in the Game Archive. Each player stores a partial Game History Object list in the finished games list identified by the name `[challenging-player-hash]|[challenge-timestamp]|[responding-player-hash]`. The object contains the game record file and player's signature of the game record file. When both players' signatures are available, the Game History Object is moved to the Game Archive.

If a player does not agree with outcome of a game they may move the game back to their `/ipns/[state]/currnet-games/` list with the special game step data object: `";C[OUTCOME-DISPUTED-PLEASE-CONTINUE]"`. The other player may then follow suit and resume play.


## Game Archive

The game archive is a collection of lists. The lists are identified by player public key hashes. Each list includes all of the Game History Objects that the player participated in. A game played by Alice and Bob would be linked in both Alice's and Bob's lists. Since the Game History Objects are identical for both players, they are only stored in disk one time. The lists are stored at `/ipns/[state]/archive/[player-hash]/` with the Game History Objects underneath. The Game History Objects are identified by their IPFS hashes. 

### Game History Object

The Game History Object is a list that points to three objects in the following order:

 * Standardized [Smart Game Format (SGF)](http://www.red-bean.com/sgf/index.html) game record file
 * The first player's signature of the game record file
 * The second player's signature of the game record file
 
The SGF version 4 format supports [40 games](http://www.red-bean.com/sgf/properties.html#GM) in its standard specification and can probably be expanded to other turn based games without difficulty.
 
A Game History Object must contain the signatures of both players to be considered valid. 

### Standardized SGF file

How to standardize the contents of this file? What are all of the required nodes? There probably aren't arbitrarily optional nodes, just nodes that are required based on circumstances in the source Current Game Record.

Things to definitely include:

 * first commit timestamp
 * last commit timestamp
 * last current game record commit, if it exists (the game might be imported from a different system)
 * each player's public key hash
 * their ratings when the game started (the ones claimed in the challenge and acceptance commits)


## Player Database

The player database is stored in the `/ipns/[state]/players` list. The data contains the last timestamp of the last modification to this list. The links point to the latest player data objects for each player that the node is tracking (including the owner), named by the `[player-public-key-hash]`.

### Player Data Object

```
{
	"links": [
		{ "hash": "[author-public-key-hash]", "size": [author-public-key-size],
		  "name": "author-public-key"
		},
		{ "hash": "[player-public-key-hash]", "size": [player-public-key-size],
		  "name": "player-public-key"
		},
		{ "hash": "[previous-version-hash]", "size": [previous-version-size],
		  "name": "previous-version"
		}
	],
	"data": {
		"timestamp": [data-timestamp],
		"name": "Friendly Name",
		"current-game-records": [
			"[current-game-record-1-head-hash]",
			"[current-game-record-2-head-hash]",
			"[current-game-record-3-head-hash]"
		],
		"game-history-objects": [
			"[game-history-object-1-hash]",
			"[game-history-object-2-hash]",
			"[game-history-object-3-hash]",
		],
		"estimated-rating": {
			"R": 1500,
			"RD": 300
		},
		"trust-coefficient": [trust-coefficient],
		"claimed-rating": {
			"R": 1475,
			"RD": 40
		},
		"ratings-by-others": {
			"[other-player-1-public-key-hash]": {
				"R": 1235,
				"RD": 20,
				"timestamp": [timestamp-of-rating]
			},
			"[other-player-2-public-key-hash]": {
				"R": 1300,
				"RD": 21,
				"timestamp": [timestamp-of-rating]
			}
		},
		"final-adjusted-rating": {
			"R": 1400,
			"RD": 80
		},
		"flags": {
			"failed-to-sign-lost-game": 1,
			"failed-to-sign-won-game": 2,
			"toxic-comments": 3
		}
	}
}
```

The details of using this structure are still sketchy. Will probably need to generate a bunch of these by making some game playing programs play games, and then figure out the way to deal with the ratings. Particularly the way the `trust-coefficient` is used.


## Node-to-Node Communications

something fancy, for sure.

## Entity Private Storage

Shared among the nodes associated with one identity. 


## Node Private

Storage Per-node configuration.


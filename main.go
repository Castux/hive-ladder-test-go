package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"testbot/arena"
)

// A basic forager: bees fetch flowers from the nearest visible field and
// bring them back to the nearest hive, exploring at random when no field is
// in sight; hives spawn bees up to maxBees. Paths are shortest paths (BFS).

const maxBees = 8

var dirs = []arena.Direction{arena.E, arena.SE, arena.SW, arena.W, arena.NW, arena.NE}

// walkable: known terrain that a bee can enter (ignoring entities)
func walkable(state *arena.GameState, c arena.Coords) bool {
	hex, ok := state.Hexes[c]
	return ok && hex.Terrain.IsWalkable()
}

// firstStep returns the direction of the first step of a shortest path from
// `from` to the nearest hex satisfying `goal`, avoiding blocked hexes
func firstStep(state *arena.GameState, from arena.Coords, blocked map[arena.Coords]bool, goal func(arena.Coords) bool) (arena.Direction, bool) {
	type node struct {
		c     arena.Coords
		first arena.Direction
	}
	seen := map[arena.Coords]bool{from: true}
	queue := []node{}
	for _, d := range dirs {
		n := from.Neighbour(d)
		if walkable(state, n) && !seen[n] {
			seen[n] = true
			queue = append(queue, node{n, d})
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if goal(cur.c) {
			if blocked[from.Neighbour(cur.first)] {
				return "", false
			}
			return cur.first, true
		}
		if blocked[cur.c] {
			continue // can be a goal neighbour, but not walked through
		}
		for _, d := range dirs {
			n := cur.c.Neighbour(d)
			if walkable(state, n) && !seen[n] {
				seen[n] = true
				queue = append(queue, node{n, cur.first})
			}
		}
	}
	return "", false
}

func think(state *arena.GameState, player int) []arena.Order {
	// Pretend to compute something more: 20 to 60 ms per turn
	time.Sleep(time.Duration(20+rand.Intn(40)) * time.Millisecond)

	var orders []arena.Order
	blocked := make(map[arena.Coords]bool) // entities, and hexes our bees move to this turn
	var bees, hives []arena.Coords
	for c, hex := range state.Hexes {
		if hex.Entity == nil {
			continue
		}
		blocked[c] = true
		if hex.Entity.Player == player {
			switch hex.Entity.Type {
			case arena.BEE:
				bees = append(bees, c)
			case arena.HIVE:
				hives = append(hives, c)
			}
		}
	}
	isOwnHive := func(c arena.Coords) bool {
		e := state.Hexes[c].Entity
		return e != nil && e.Type == arena.HIVE && e.Player == player
	}
	nextToHive := func(c arena.Coords) bool {
		for _, n := range c.Neighbours() {
			if hex, ok := state.Hexes[n]; ok && hex.Entity != nil && isOwnHive(n) {
				return true
			}
		}
		return false
	}
	freeField := func(c arena.Coords) bool {
		hex := state.Hexes[c]
		return hex.Terrain == arena.FIELD && hex.Resources > 0 && !blocked[c]
	}

	for _, c := range bees {
		bee := state.Hexes[c].Entity
		hex := state.Hexes[c]
		move := func(d arena.Direction) {
			orders = append(orders, arena.Order{Type: arena.MOVE, Coords: c, Direction: d})
			delete(blocked, c)
			blocked[c.Neighbour(d)] = true
		}
		switch {
		case bee.HasFlower && nextToHive(c):
			orders = append(orders, arena.Order{Type: arena.FORAGE, Coords: c})
		case bee.HasFlower:
			if d, ok := firstStep(state, c, blocked, nextToHive); ok {
				move(d)
			}
		case hex.Terrain == arena.FIELD && hex.Resources > 0:
			orders = append(orders, arena.Order{Type: arena.FORAGE, Coords: c})
		default:
			if d, ok := firstStep(state, c, blocked, freeField); ok {
				move(d)
			} else {
				// Explore: a random free direction
				for _, i := range rand.Perm(len(dirs)) {
					if n := c.Neighbour(dirs[i]); walkable(state, n) && !blocked[n] {
						move(dirs[i])
						break
					}
				}
			}
		}
	}

	resources := state.PlayerResources[player]
	count := len(bees)
	for _, h := range hives {
		if count >= maxBees || resources < state.Rules.BeeCost {
			break
		}
		for _, i := range rand.Perm(len(dirs)) {
			if n := h.Neighbour(dirs[i]); walkable(state, n) && !blocked[n] {
				orders = append(orders, arena.Order{Type: arena.SPAWN, Coords: h, Direction: dirs[i]})
				blocked[n] = true
				resources -= state.Rules.BeeCost
				count++
				break
			}
		}
	}

	if state.Turn%100 == 0 {
		fmt.Printf("Turn %d: %d bees, %d flowers\n", state.Turn, len(bees), state.PlayerResources[player])
	}
	return orders
}

func main() {
	// Arguments, or environment variables when run by the ladder (see docs/docker.md)
	var host, id, name string
	if len(os.Args) > 3 {
		host, id, name = os.Args[1], os.Args[2], os.Args[3]
	} else {
		host, id, name = os.Getenv("ARENA_HOST"), os.Getenv("ARENA_GAME_ID"), os.Getenv("ARENA_NAME")
	}

	if host == "" || id == "" || name == "" {
		fmt.Println("Usage: ./agent <host> <gameid> <name>")
		fmt.Println("or set the ARENA_HOST, ARENA_GAME_ID and ARENA_NAME environment variables")
		os.Exit(1)
	}

	Run(host, id, name, think)
}

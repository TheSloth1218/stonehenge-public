package generator

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"stonehenge-targets/internal/model"
)

type Generator struct {
	mu       sync.Mutex
	sourceID string
	rand     *rand.Rand
	nextID   uint64
}

func New(sourceID string, seed *int64) *Generator {
	var seedValue int64
	if seed != nil {
		seedValue = *seed
	} else {
		seedValue = time.Now().UnixNano()
	}
	return &Generator{
		sourceID: sourceID,
		rand:     rand.New(rand.NewSource(seedValue)),
	}
}

func (g *Generator) Next(now time.Time) model.FragmentTrack {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nextID++
	mapX, mapY, terrain, zone := RandomImpact(g.rand)
	return model.FragmentTrack{
		ExternalID:          fmt.Sprintf("%s-%06d", g.sourceID, g.nextID),
		SourceID:            g.sourceID,
		Bearing:             float64(g.rand.Intn(360)),
		Elevation:           5 + g.rand.Float64()*80,
		MapX:                mapX,
		MapY:                mapY,
		Priority:            PriorityFor(terrain, zone, g.rand),
		TerrainType:         terrain,
		StrategicZone:       zone,
		TimeToImpactSeconds: 45 + g.rand.Intn(556),
		ObservedAt:          now.UTC(),
	}
}

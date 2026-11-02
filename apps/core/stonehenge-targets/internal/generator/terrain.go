package generator

import (
	"math"
	"math/rand"

	"stonehenge-targets/internal/model"
)

type Point struct {
	X float64
	Y float64
}

type Zone struct {
	Name    string
	Polygon []Point
}

var landZones = []Zone{
	{Name: "MAIN_CONTINENT", Polygon: []Point{{10, 30}, {16, 22}, {25, 20}, {35, 27}, {44, 30}, {51, 14}, {62, 9}, {71, 10}, {77, 20}, {75, 31}, {85, 25}, {83, 38}, {74, 48}, {70, 58}, {61, 62}, {60, 82}, {53, 88}, {47, 75}, {43, 67}, {35, 68}, {31, 59}, {24, 57}, {16, 49}, {9, 42}}},
	{Name: "SOUTHERN_PENINSULA", Polygon: []Point{{52, 70}, {60, 75}, {59, 88}, {53, 91}, {48, 81}}},
	{Name: "FARBANTI_ISLANDS", Polygon: []Point{{9, 34}, {15, 34}, {16, 43}, {10, 46}, {6, 40}}},
	{Name: "RIGLEY_ARCHIPELAGO", Polygon: []Point{{72, 54}, {78, 51}, {83, 59}, {77, 68}, {70, 65}}},
	{Name: "NORTH_POINT", Polygon: []Point{{76, 25}, {84, 22}, {89, 30}, {84, 38}, {76, 35}}},
	{Name: "SOUTHEAST_ISLANDS", Polygon: []Point{{78, 67}, {88, 60}, {91, 69}, {85, 81}, {76, 77}}},
}

var strategicPoints = []struct {
	Name   string
	X      float64
	Y      float64
	Radius float64
}{
	{Name: "STONEHENGE", X: 29, Y: 51, Radius: 5},
	{Name: "FARBANTI", X: 14, Y: 39, Radius: 8},
	{Name: "SAN_SALVACION", X: 30, Y: 43, Radius: 9},
	{Name: "LOS_CANAS", X: 57, Y: 59, Radius: 8},
	{Name: "RIGLEY_AIR_BASE", X: 64, Y: 42, Radius: 9},
	{Name: "NEW_FIELDS", X: 80, Y: 34, Radius: 8},
	{Name: "NORTH_POINT", X: 83, Y: 29, Radius: 9},
}

func Classify(x, y float64) (model.TerrainType, string) {
	for _, zone := range landZones {
		if pointInPolygon(Point{X: x, Y: y}, zone.Polygon) {
			return model.TerrainLand, strategicZoneFor(x, y, zone.Name)
		}
	}
	return model.TerrainSea, "OPEN_SEA"
}

func PriorityFor(terrain model.TerrainType, zone string, r *rand.Rand) model.Priority {
	if terrain == model.TerrainSea {
		if r.Intn(100) < 8 {
			return model.PriorityMedium
		}
		return model.PriorityLow
	}
	if zone == "STONEHENGE" || zone == "FARBANTI" || zone == "SAN_SALVACION" || zone == "NORTH_POINT" {
		if r.Intn(100) < 45 {
			return model.PriorityCritical
		}
		return model.PriorityHigh
	}
	roll := r.Intn(100)
	switch {
	case roll < 18:
		return model.PriorityCritical
	case roll < 82:
		return model.PriorityHigh
	default:
		return model.PriorityMedium
	}
}

func RandomImpact(r *rand.Rand) (float64, float64, model.TerrainType, string) {
	wantLand := r.Intn(100) < 84
	for i := 0; i < 80; i++ {
		x := 6 + r.Float64()*88
		y := 9 + r.Float64()*82
		terrain, zone := Classify(x, y)
		if (wantLand && terrain == model.TerrainLand) || (!wantLand && terrain == model.TerrainSea) {
			return x, y, terrain, zone
		}
	}
	x := 9 + r.Float64()*78
	y := 12 + r.Float64()*76
	terrain, zone := Classify(x, y)
	return x, y, terrain, zone
}

func strategicZoneFor(x, y float64, fallback string) string {
	bestName := fallback
	bestDistance := math.MaxFloat64
	for _, point := range strategicPoints {
		distance := math.Hypot(x-point.X, y-point.Y)
		if distance <= point.Radius && distance < bestDistance {
			bestDistance = distance
			bestName = point.Name
		}
	}
	return bestName
}

func pointInPolygon(point Point, polygon []Point) bool {
	inside := false
	j := len(polygon) - 1
	for i := range polygon {
		xi, yi := polygon[i].X, polygon[i].Y
		xj, yj := polygon[j].X, polygon[j].Y
		intersects := ((yi > point.Y) != (yj > point.Y)) && (point.X < (xj-xi)*(point.Y-yi)/(yj-yi)+xi)
		if intersects {
			inside = !inside
		}
		j = i
	}
	return inside
}

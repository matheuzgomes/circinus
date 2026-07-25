package document

type Shape string

const (
	ShapeBox      Shape = "box"
	ShapeDatabase Shape = "database"
	ShapeGateway  Shape = "gateway"
	ShapeQueue    Shape = "queue"
)

type ShapeDefinition struct {
	Shape         Shape
	Label         string
	DefaultWidth  int
	DefaultHeight int
	LabelInset    int
	LabelRow      int
}

var shapeCatalog = []ShapeDefinition{
	{Shape: ShapeBox, Label: "Box", DefaultWidth: 18, DefaultHeight: 5, LabelInset: 1, LabelRow: 2},
	{Shape: ShapeDatabase, Label: "Database", DefaultWidth: 18, DefaultHeight: 10, LabelInset: 2, LabelRow: 5},
	{Shape: ShapeGateway, Label: "Gateway", DefaultWidth: 18, DefaultHeight: 5, LabelInset: 1, LabelRow: 2},
	{Shape: ShapeQueue, Label: "Queue", DefaultWidth: 18, DefaultHeight: 5, LabelInset: 1, LabelRow: 1},
}

func ShapeDefinitions() []ShapeDefinition {
	return append([]ShapeDefinition(nil), shapeCatalog...)
}

func SpecializedShapeDefinitions() []ShapeDefinition {
	shapes := make([]ShapeDefinition, 0, len(shapeCatalog)-1)
	for _, definition := range shapeCatalog {
		if definition.Shape != ShapeBox {
			shapes = append(shapes, definition)
		}
	}
	return shapes
}

func ShapeDefinitionFor(shape Shape) (ShapeDefinition, bool) {
	if shape == "" {
		shape = ShapeBox
	}
	for _, definition := range shapeCatalog {
		if definition.Shape == shape {
			return definition, true
		}
	}
	return ShapeDefinition{}, false
}

func EffectiveShape(element Element) Shape {
	if element.Shape == "" {
		return ShapeBox
	}
	return Shape(element.Shape)
}

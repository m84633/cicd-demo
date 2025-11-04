package snowflake

import (
	"fmt"
	"time"

	"github.com/sony/sonyflake"
)

// Generator holds the sonyflake instance.
type Generator struct {
	node *sonyflake.Sonyflake
}

// NewGenerator creates and returns a new Generator.
func NewGenerator(machineId uint16) (*Generator, error) {
	t, _ := time.Parse("2006-01-02", "2020-01-01")
	settings := sonyflake.Settings{
		StartTime: t,
		MachineID: func() (uint16, error) { // machineId is captured by the closure
			return machineId, nil
		},
	}
	sf := sonyflake.NewSonyflake(settings)
	if sf == nil {
		return nil, fmt.Errorf("sonyflake not created")
	}
	return &Generator{node: sf}, nil
}

// GetID generates a new unique id.
func (g *Generator) GetID() (uint64, error) {
	return g.node.NextID()
}

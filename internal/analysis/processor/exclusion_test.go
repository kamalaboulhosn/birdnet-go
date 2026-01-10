package processor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/datastore"
)

func TestSpeciesExclusion(t *testing.T) {
	// Setup settings
	settings := &conf.Settings{
		BirdNET: conf.BirdNETSettings{
			Threshold: 0.5,
			RangeFilter: conf.RangeFilterSettings{
				// Species must be in this list to pass the inclusion filter (whitelist)
				Species: []string{"Excluded Bird"},
			},
		},
		Realtime: conf.RealtimeSettings{
			Species: conf.SpeciesSettings{
				// Species in this list should be filtered out (blacklist)
				Exclude: []string{"Excluded Bird"},
			},
		},
	}

	// Minimal BirdNET instance (interpreters can be nil as we rely on mocked behavior or valid defaults)
	bn := &birdnet.BirdNET{
		Settings: settings,
	}

	p := &Processor{
		Settings: settings,
		Bn:       bn,
	}

	// Create a result item mimicking what comes from detection
	detectionResult := datastore.Results{
		Species:    "Excluded Bird",
		Confidence: 0.9,
	}
	
	results := birdnet.Results{
		Results: []datastore.Results{detectionResult},
		Source:  datastore.Source{ID: "test_source"},
	}

	// Process results
    // processResults is private, but we are in package processor so we can call it
	detections := p.processResults(results)

	// Assertions
    // Current behavior (Bug): Detection passes through because Exclude list is ignored.
    // Desired behavior (Fix): Detection is filtered out.
	assert.Empty(t, detections, "Should exclude species listed in exclude list")
}

func TestSpeciesExclusion_IncludedSpecies(t *testing.T) {
	// Verify that non-excluded species still pass through
	settings := &conf.Settings{
		BirdNET: conf.BirdNETSettings{
			Threshold: 0.5,
			RangeFilter: conf.RangeFilterSettings{
				Species: []string{"Included Bird", "Excluded Bird"},
			},
		},
		Realtime: conf.RealtimeSettings{
			Species: conf.SpeciesSettings{
				Exclude: []string{"Excluded Bird"},
			},
		},
	}

	bn := &birdnet.BirdNET{
		Settings: settings,
	}

	p := &Processor{
		Settings: settings,
		Bn:       bn,
	}

	detectionResult := datastore.Results{
		Species:    "Included Bird",
		Confidence: 0.9,
	}
	
	results := birdnet.Results{
		Results: []datastore.Results{detectionResult},
		Source:  datastore.Source{ID: "test_source"},
	}

	detections := p.processResults(results)

	assert.NotEmpty(t, detections, "Should NOT exclude species NOT listed in exclude list")
	if len(detections) > 0 {
		assert.Equal(t, "Included Bird", detections[0].Species.ScientificName, "Scientific name should match")
	}
}

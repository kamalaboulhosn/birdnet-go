package processor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tphakala/birdnet-go/internal/analysis/jobqueue"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/datastore"
)

func TestMqttAction_Cooldown(t *testing.T) {
	// Create test note
	speciesName := "Blue Jay"
	testNote := datastore.Note{
		CommonName:     speciesName,
		ScientificName: "Cyanocitta cristata",
		Confidence:     0.98,
		ClipName:       "blue_jay.wav",
		Date:           "2024-01-15",
		Time:           "10:00:00",
		Source:         testAudioSource(),
	}

	// Create mock MQTT client
	mockClient := &MockMqttClientWithCapture{
		Connected: true,
	}

	// Create event tracker
	eventTracker := NewEventTracker(60 * time.Second)

	// Create test settings with CooldownMinutes = 5
	settings := &conf.Settings{
		Realtime: conf.RealtimeSettings{
			MQTT: conf.MQTTSettings{
				Enabled:         true,
				Topic:           "birdnet/detections",
				CooldownMinutes: 5,
			},
		},
		Debug: true,
	}

	// Create a dummy Processor instance with map initialized
	proc := &Processor{
		Settings:             settings,
		LastMqttNotification: make(map[string]time.Time),
	}

	// Create MQTT action with injected processor
	action := &MqttAction{
		Settings:       settings,
		Note:           testNote,
		BirdImageCache: nil,
		MqttClient:     mockClient,
		EventTracker:   eventTracker,
		RetryConfig:    jobqueue.RetryConfig{Enabled: false},
		processor:      proc,
	}

	// 1. First execution - should publish
	err := action.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "birdnet/detections", mockClient.PublishedTopic)
	assert.NotEmpty(t, mockClient.PublishedData)
	
	// Verify timestamp was recorded
	proc.detectionMutex.Lock() // Although single threaded test, good practice since logic uses lock
	lastTime, exists := proc.LastMqttNotification[speciesName]
	proc.detectionMutex.Unlock()
	assert.True(t, exists)
	assert.WithinDuration(t, time.Now(), lastTime, 1*time.Second)

	// Reset mock
	mockClient.PublishedTopic = ""
	mockClient.PublishedData = ""

	// 2. Second execution immediately - should NOT publish due to cooldown
	err = action.Execute(nil)
	require.NoError(t, err)
	assert.Empty(t, mockClient.PublishedTopic) 
	assert.Empty(t, mockClient.PublishedData)

	// 3. Simulate time passing (move last notification back 6 minutes)
	proc.detectionMutex.Lock()
	proc.LastMqttNotification[speciesName] = time.Now().Add(-6 * time.Minute)
	proc.detectionMutex.Unlock()

	// 4. Third execution - should publish again
	err = action.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "birdnet/detections", mockClient.PublishedTopic)
	assert.NotEmpty(t, mockClient.PublishedData)
	
	// Verify timestamp was updated
	proc.detectionMutex.Lock()
	newLastTime, _ := proc.LastMqttNotification[speciesName]
	proc.detectionMutex.Unlock()
	assert.True(t, newLastTime.After(lastTime))
	assert.WithinDuration(t, time.Now(), newLastTime, 1*time.Second)
}

func TestMqttAction_CooldownDisabled(t *testing.T) {
	// Create test note
	speciesName := "Cardinal"
	testNote := datastore.Note{
		CommonName:     speciesName,
		Confidence:     0.98,
		Source:         testAudioSource(),
	}

	mockClient := &MockMqttClientWithCapture{Connected: true}
	eventTracker := NewEventTracker(60 * time.Second)

	// Settings with CooldownMinutes = 0 (default/disabled)
	settings := &conf.Settings{
		Realtime: conf.RealtimeSettings{
			MQTT: conf.MQTTSettings{
				Enabled:         true,
				Topic:           "birdnet/detections",
				CooldownMinutes: 0,
			},
		},
	}

	proc := &Processor{
		Settings:             settings,
		LastMqttNotification: make(map[string]time.Time),
	}

	action := &MqttAction{
		Settings:       settings,
		Note:           testNote,
		MqttClient:     mockClient,
		EventTracker:   eventTracker,
		processor:      proc,
	}

	// 1. First execution
	err := action.Execute(nil)
	require.NoError(t, err)
	assert.NotEmpty(t, mockClient.PublishedData)

	// Reset mock
	mockClient.PublishedData = ""

	// 2. Second execution immediately - should STILL publish because cooldown is disabled
	err = action.Execute(nil)
	require.NoError(t, err)
	assert.NotEmpty(t, mockClient.PublishedData)
}

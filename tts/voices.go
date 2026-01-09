package tts

import (
	"fmt"
	"os"
	"path/filepath"
)

// VoiceMapping maps voice names to Supertonic voice style files
var VoiceMapping = map[string]string{
	"M1":  "assets/voice_styles/M1.json",
	"M2":  "assets/voice_styles/M2.json",
	"M3":  "assets/voice_styles/M3.json",
	"M4":  "assets/voice_styles/M4.json",
	"M5":  "assets/voice_styles/M5.json",
	"F1":  "assets/voice_styles/F1.json",
	"F2":  "assets/voice_styles/F2.json",
	"F3":  "assets/voice_styles/F3.json",
	"F4":  "assets/voice_styles/F4.json",
	"F5":  "assets/voice_styles/F5.json",
}

// GetVoicePath returns the Supertonic voice style path for an OpenAI voice name
func GetVoicePath(voiceName string) (string, error) {
	// First check if the file exists in the default location
	path, exists := VoiceMapping[voiceName]
	if !exists {
		return "", fmt.Errorf("unsupported voice: %s. Available voices: %v",
			voiceName, GetAvailableVoices())
	}

	// Check if file exists in default location
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// ToDo: make this path configurable
	// Check if file exists in /var/lib/supertonic
	altPath := filepath.Join("/var/lib/supertonic", path)
	if _, err := os.Stat(altPath); err == nil {
		return altPath, nil
	}

	return "", fmt.Errorf("voice style file not found for %s in either location", voiceName)
}

// GetAvailableVoices returns list of voice names
func GetAvailableVoices() []string {
	voices := make([]string, 0, len(VoiceMapping))
	for voice := range VoiceMapping {
		voices = append(voices, voice)
	}
	return voices
}

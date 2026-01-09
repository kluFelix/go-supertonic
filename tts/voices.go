package tts

import (
	"fmt"
	"os"
	"path/filepath"
)

// VoiceMapping maps voice names to Supertonic voice style filenames
// The actual path will be constructed as [assetsDir]/voice_styles/[filename]
var VoiceMapping = map[string]string{
	"M1": "M1.json",
	"M2": "M2.json",
	"M3": "M3.json",
	"M4": "M4.json",
	"M5": "M5.json",
	"F1": "F1.json",
	"F2": "F2.json",
	"F3": "F3.json",
	"F4": "F4.json",
	"F5": "F5.json",
}

// GetVoicePath returns the full path to a voice style file given the voice name and assets directory
func GetVoicePath(voiceName string, assetsDir string) (string, error) {
	filename, exists := VoiceMapping[voiceName]
	if !exists {
		return "", fmt.Errorf("unsupported voice: %s. Available voices: %v",
			voiceName, GetAvailableVoices())
	}

	path := filepath.Join(assetsDir, "voice_styles", filename)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("voice style file not found for %s at %s", voiceName, path)
	}

	return path, nil
}

// GetAvailableVoices returns list of available voice names
func GetAvailableVoices() []string {
	voices := make([]string, 0, len(VoiceMapping))
	for voice := range VoiceMapping {
		voices = append(voices, voice)
	}
	return voices
}

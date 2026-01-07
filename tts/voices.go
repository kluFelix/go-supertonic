package tts

import "fmt"

// VoiceMapping maps OpenAI voice names to Supertonic voice style files
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
	path, exists := VoiceMapping[voiceName]
	if !exists {
		return "", fmt.Errorf("unsupported voice: %s. Available voices: %v", 
			voiceName, GetAvailableVoices())
	}
	return path, nil
}

// GetAvailableVoices returns list of OpenAI-compatible voice names
func GetAvailableVoices() []string {
	voices := make([]string, 0, len(VoiceMapping))
	for voice := range VoiceMapping {
		voices = append(voices, voice)
	}
	return voices
}

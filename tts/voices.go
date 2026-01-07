package tts

import "fmt"

// VoiceMapping maps OpenAI voice names to Supertonic voice style files
var VoiceMapping = map[string]string{
	"alloy":  "assets/voice_styles/M1.json",
	"echo":   "assets/voice_styles/M2.json",
	"fable":  "assets/voice_styles/F1.json",
	"onyx":   "assets/voice_styles/F2.json",
	"nova":   "assets/voice_styles/M3.json",  // Need additional voices
	"shimmer": "assets/voice_styles/F3.json", // Need additional voices
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

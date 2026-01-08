package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	ort "github.com/yalue/onnxruntime_go"
	"go-supertonic/tts"
)

// TTSRequest with OpenAI TTS request structure
type TTSRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"` // mp3, opus, aac, flac, wav, pcm
	Speed          float64 `json:"speed"`           // 0.25 to 4.0
}

// ServerConfig with API server configuration
type ServerConfig struct {
	Port         string
	OnnxDir      string
	UseGPU       bool
	TotalStep    int
	DefaultSpeed float64
	SaveDir      string
}

var config ServerConfig

func main() {
	// Parse command-line flags
	flag.StringVar(&config.Port, "port", "8880", "Server port")
	flag.StringVar(&config.OnnxDir, "onnx-dir", "assets/onnx", "Path to ONNX model directory")
	flag.BoolVar(&config.UseGPU, "use-gpu", false, "Use GPU for inference")
	flag.IntVar(&config.TotalStep, "total-step", 5, "Number of denoising steps (quality vs speed)")
	flag.Float64Var(&config.DefaultSpeed, "default-speed", 1.0, "Default speech speed")
	flag.Parse()

	// Initialize ONNX Runtime
	fmt.Println("=== Supertonic OpenAI-Compatible TTS API ===")
	fmt.Printf("Initializing ONNX Runtime...\n")
	if err := tts.InitializeONNXRuntime(); err != nil {
		log.Fatalf("Failed to initialize ONNX Runtime: %v", err)
	}
	defer ort.DestroyEnvironment()

	// Verify assets exist
	if err := verifyAssets(); err != nil {
		log.Fatalf("Asset verification failed: %v", err)
	}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/audio/speech", handleTTSRequest)
	mux.HandleFunc("/health", handleHealthCheck)
	mux.HandleFunc("/", handleRoot)

	// Start server
	addr := ":" + config.Port
	fmt.Printf("\nServer starting on http://localhost%s\n", addr)
	fmt.Printf("Endpoint: POST /v1/audio/speech\n")
	fmt.Printf("Voices: %v\n", tts.GetAvailableVoices())
	fmt.Printf("Response Formats: wav (native), mp3, opus, aac, flac, pcm\n\n")

	log.Fatal(http.ListenAndServe(addr, mux))
}

// verifyAssets checks if required model files exist
func verifyAssets() error {
	requiredFiles := []string{
		filepath.Join(config.OnnxDir, "duration_predictor.onnx"),
		filepath.Join(config.OnnxDir, "text_encoder.onnx"),
		filepath.Join(config.OnnxDir, "vector_estimator.onnx"),
		filepath.Join(config.OnnxDir, "vocoder.onnx"),
		filepath.Join(config.OnnxDir, "tts.json"),
		filepath.Join(config.OnnxDir, "unicode_indexer.json"),
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("missing required file: %s", file)
		}
	}

	// Check voice styles
	for _, voiceFile := range tts.VoiceMapping {
		if _, err := os.Stat(voiceFile); os.IsNotExist(err) {
			log.Printf("Warning: Missing voice style %s", voiceFile)
		}
	}

	return nil
}

// handleRoot provides API documentation
func handleRoot(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request to root")
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"message": "Supertonic OpenAI-Compatible TTS API",
		"endpoints": map[string]string{
			"POST /v1/audio/speech": "Generate speech from text",
			"GET /health":           "Health check",
		},
		"voices":           tts.GetAvailableVoices(),
		"models":           []string{"tts-1", "tts-1-hd"},
		"response_formats": []string{"wav", "mp3", "opus", "aac", "flac", "pcm"},
	}
	json.NewEncoder(w).Encode(response)
}

// handleHealthCheck returns service health
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "supertonic-tts",
	})
}

// handleTTSRequest processes OpenAI-compatible TTS requests
func handleTTSRequest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		log.Printf("Invalid method")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req TTSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid JSON")
		sendError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateRequest(&req); err != nil {
		sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log request
	log.Printf("TTS Request: voice=%s, model=%s, format=%s, speed=%.2f, text=\"%.50s\"",
		req.Voice, req.Model, req.ResponseFormat, req.Speed, req.Input)

	// Generate speech
	audioData, err := generateSpeech(&req)
	if err != nil {
		log.Printf("TTS Error: %v", err)
		sendError(w, "Speech generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	setAudioHeaders(w, req.ResponseFormat)

	// Write audio data
	w.Write(audioData)
}

// validateRequest checks if the request is valid
func validateRequest(req *TTSRequest) error {
	if req.Input == "" {
		return fmt.Errorf("input text is required")
	}

	if req.Voice == "" {
		req.Voice = "F5" // Default voice
	}

	if req.Model == "" {
		req.Model = "tts-1" // Default model
	}

	if req.ResponseFormat == "" {
		req.ResponseFormat = "wav" // Default format
	}

	if req.Speed == 0 {
		req.Speed = config.DefaultSpeed
	}

	// Validate voice
	if _, err := tts.GetVoicePath(req.Voice); err != nil {
		return err
	}

	// Validate model
	if req.Model != "tts-1" && req.Model != "tts-1-hd" {
		return fmt.Errorf("unsupported model: %s (use 'tts-1' or 'tts-1-hd')", req.Model)
	}

	// Validate speed (OpenAI allows 0.25 to 4.0)
	if req.Speed < 0.25 || req.Speed > 4.0 {
		return fmt.Errorf("speed must be between 0.25 and 4.0")
	}

	// Validate response format
	validFormats := map[string]bool{
		"wav": true, "mp3": true, "opus": true,
		"aac": true, "flac": true, "pcm": true,
	}
	if !validFormats[req.ResponseFormat] {
		return fmt.Errorf("unsupported response format: %s", req.ResponseFormat)
	}

	return nil
}

// generateSpeech generates speech from the request
func generateSpeech(req *TTSRequest) ([]byte, error) {
	// Load config
	cfg, err := tts.LoadCfgs(config.OnnxDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Load TTS components
	textToSpeech, err := tts.LoadTextToSpeech(config.OnnxDir, config.UseGPU, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load TTS: %w", err)
	}
	defer textToSpeech.Destroy()

	// Get voice style path
	voicePath, err := tts.GetVoicePath(req.Voice)
	if err != nil {
		return nil, err
	}

	// Load voice style
	style, err := tts.LoadVoiceStyle([]string{voicePath}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load voice style: %w", err)
	}
	defer style.Destroy()

	// Determine quality (model affects steps)
	totalStep := config.TotalStep
	if req.Model == "tts-1-hd" {
		totalStep = 10 // Higher quality for HD model
	}

	// Generate speech (language detection could be added here)
	language := "en" // Default to English
	// TODO: Add language detection or allow language specification

	fmt.Printf("Generating speech (model=%s, steps=%d, speed=%.2f)...\n",
		req.Model, totalStep, req.Speed)

	var wav []float32
	var duration float32

	// Generate using the Call method (handles chunking)
	wav, duration, err = textToSpeech.Call(req.Input, language, style, totalStep, float32(req.Speed), 0.3)
	if err != nil {
		return nil, fmt.Errorf("speech generation failed: %w", err)
	}

	// Convert to bytes
	audioData, err := convertToFormat(wav, textToSpeech.SampleRate, req.ResponseFormat)
	if err != nil {
		return nil, fmt.Errorf("format conversion failed: %w", err)
	}

	log.Printf("Generated audio: %d bytes, duration: %.2fs", len(audioData), duration)
	return audioData, nil
}

// setAudioHeaders sets appropriate HTTP headers for audio responses
func setAudioHeaders(w http.ResponseWriter, format string) {
	w.Header().Set("Content-Type", getContentType(format))
	// Optional: Add CORS headers if needed
	// w.Header().Set("Access-Control-Allow-Origin", "*")
}

// getContentType returns MIME type for audio format
func getContentType(format string) string {
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "opus":
		return "audio/opus"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	case "pcm":
		return "audio/L16;rate=24000" // 16-bit linear PCM
	default:
		return "audio/wav"
	}
}

// sendError sends JSON error response
func sendError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// convertToFormat converts raw audio to requested format
func convertToFormat(wav []float32, sampleRate int, format string) ([]byte, error) {
	// For now, we return WAV natively
	// For other formats, you'll need to use external tools like ffmpeg
	// or Go libraries like github.com/hajimehoshi/go-mp3

	if format == "wav" {
		return wavToBytes(wav, sampleRate), nil
	}

	// TODO: Implement format conversion or use external encoder
	return nil, fmt.Errorf("format '%s' not implemented yet. Use 'wav' for now", format)
}

// wavToBytes converts float32 WAV data to WAV file bytes using a temporary file
func wavToBytes(audioData []float32, sampleRate int) []byte {
	// Create a temporary file (implements io.WriteSeeker)
	tmpfile, err := os.CreateTemp("", "supertonic-*.wav")
	if err != nil {
		log.Printf("Error creating temp file: %v", err)
		return nil
	}
	defer os.Remove(tmpfile.Name()) // Clean up after reading
	defer tmpfile.Close()

	// Create WAV encoder with the temp file
	encoder := wav.NewEncoder(tmpfile, sampleRate, 16, 1, 1)

	// Convert float32 to int16
	data := make([]int, len(audioData))
	for i, sample := range audioData {
		clamped := float64(sample)
		if clamped > 1.0 {
			clamped = 1.0
		} else if clamped < -1.0 {
			clamped = -1.0
		}
		data[i] = int(clamped * 32767)
	}

	// Write audio data
	audioBuf := &audio.IntBuffer{
		Data:           data,
		Format:         &audio.Format{SampleRate: sampleRate, NumChannels: 1},
		SourceBitDepth: 16,
	}
	encoder.Write(audioBuf)
	encoder.Close()

	// Seek back to beginning and read the file
	tmpfile.Seek(0, 0)
	bytes, err := io.ReadAll(tmpfile)
	if err != nil {
		log.Printf("Error reading temp file: %v", err)
		return nil
	}

	return bytes
}

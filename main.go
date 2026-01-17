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
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	Speed          float64 `json:"speed"`
}

// ServerConfig with API server configuration
type ServerConfig struct {
	Port         string
	AssetsDir    string
	UseGPU       bool
	TotalStep    int
	DefaultSpeed float64
	SaveDir      string
}

var config ServerConfig

func main() {
	// Parse command-line flags
	var assetsDir string
	flag.StringVar(&config.Port, "port", "8880", "Server port")
	flag.StringVar(&assetsDir, "assets-dir", "", "Path to assets directory (optional, will auto-detect if not provided)")
	flag.BoolVar(&config.UseGPU, "use-gpu", false, "Use GPU for inference")
	flag.IntVar(&config.TotalStep, "total-step", 5, "Number of denoising steps (quality vs speed)")
	flag.Float64Var(&config.DefaultSpeed, "default-speed", 1.0, "Default speech speed")
	flag.Parse()

	// Find assets directory
	var err error
	config.AssetsDir, err = findAssetsDir(assetsDir)
	if err != nil {
		log.Fatalf("Failed to locate assets directory: %v", err)
	}

	// Initialize ONNX Runtime
	fmt.Println("=== Supertonic OpenAI-Compatible TTS API ===")
	fmt.Printf("Using assets directory: %s\n", config.AssetsDir)
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

	log.Fatal(http.ListenAndServe(addr, mux))
}

// findAssetsDir locates the assets directory based on priority:
// 1. Command-line flag (if provided)
// 2. System-wide location: /var/lib/supertonic/assets
// 3. Local directory: ./assets
func findAssetsDir(cmdLinePath string) (string, error) {
	// Priority 1: Command line flag
	if cmdLinePath != "" {
		info, err := os.Stat(cmdLinePath)
		if err != nil {
			return "", fmt.Errorf("specified assets directory not accessible: %s: %w", cmdLinePath, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("specified assets directory is not a directory: %s", cmdLinePath)
		}
		// Convert to absolute path for consistency
		absPath, err := filepath.Abs(cmdLinePath)
		if err != nil {
			return cmdLinePath, nil // Fallback to original if abs fails
		}
		return absPath, nil
	}

	// Priority 2: System-wide location
	systemPath := "/var/lib/supertonic/assets"
	if info, err := os.Stat(systemPath); err == nil && info.IsDir() {
		absPath, _ := filepath.Abs(systemPath)
		return absPath, nil
	}

	// Priority 3: Local directory (relative to working directory)
	localPath := "./assets"
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		absPath, _ := filepath.Abs(localPath)
		return absPath, nil
	}

	return "", fmt.Errorf("could not find assets directory in any default location. " +
		"Please specify the path using --assets-dir\n" +
		"Searched locations:\n" +
		"  - /var/lib/supertonic/assets\n" +
		"  - ./assets")
}

// verifyAssets checks if required model files exist in the determined assets directory
func verifyAssets() error {
	onnxDir := filepath.Join(config.AssetsDir, "onnx")
	voiceStylesDir := filepath.Join(config.AssetsDir, "voice_styles")

	// Check for ONNX model files
	requiredFiles := []string{
		"duration_predictor.onnx",
		"text_encoder.onnx",
		"vector_estimator.onnx",
		"vocoder.onnx",
		"tts.json",
		"unicode_indexer.json",
	}

	for _, file := range requiredFiles {
		path := filepath.Join(onnxDir, file)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("missing required ONNX file: %s", path)
		}
	}

	// Check voice style files
	for voiceName, filename := range tts.VoiceMapping {
		path := filepath.Join(voiceStylesDir, filename)
		if _, err := os.Stat(path); err != nil {
			log.Printf("Warning: Missing voice style %s at %s", voiceName, path)
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
	log.Printf("TTS Request: voice=%s, speed=%.2f, text=\"%.50s\"",
		req.Voice, req.Speed, req.Input)

	// Generate speech
	audioData, err := generateSpeech(&req)
	if err != nil {
		log.Printf("TTS Error: %v", err)
		sendError(w, "Speech generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set wav headers
	w.Header().Set("Content-Type", "audio/wav")

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

	if req.Speed == 0 {
		req.Speed = config.DefaultSpeed
	}

	// Validate voice
	if _, err := tts.GetVoicePath(req.Voice, config.AssetsDir); err != nil {
		return err
	}

	// Validate speed (OpenAI allows 0.25 to 4.0)
	if req.Speed < 0.25 || req.Speed > 4.0 {
		return fmt.Errorf("speed must be between 0.25 and 4.0")
	}

	return nil
}

// generateSpeech generates speech from the request
func generateSpeech(req *TTSRequest) ([]byte, error) {
	// Load config from assets directory
	cfg, err := tts.LoadCfgs(config.AssetsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Load TTS components from assets directory
	textToSpeech, err := tts.LoadTextToSpeech(config.AssetsDir, config.UseGPU, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load TTS: %w", err)
	}
	defer textToSpeech.Destroy()

	// Get voice style path
	voicePath, err := tts.GetVoicePath(req.Voice, config.AssetsDir)
	if err != nil {
		return nil, err
	}

	// Load voice style
	style, err := tts.LoadVoiceStyle([]string{voicePath}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load voice style: %w", err)
	}
	defer style.Destroy()


	// Generate speech (language detection could be added here)
	language := "en"
	fmt.Printf("Generating speech (steps=%d, speed=%.2f)...\n",
		config.TotalStep, req.Speed)

	// Generate using the Call method (handles chunking)
	wav, duration, err := textToSpeech.Call(req.Input, language, style, config.TotalStep, float32(req.Speed), 0.3)
	if err != nil {
		return nil, fmt.Errorf("speech generation failed: %w", err)
	}

	// Convert to bytes
	audioData := wavToBytes(wav, textToSpeech.SampleRate)

	log.Printf("Generated audio: %d bytes, duration: %.2fs", len(audioData), duration)
	return audioData, nil
}

// sendError sends JSON error response
func sendError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// wavToBytes converts float32 WAV data to WAV file bytes using a temporary file
func wavToBytes(audioData []float32, sampleRate int) []byte {
	// Create a temporary file (implements io.WriteSeeker)
	tmpfile, err := os.CreateTemp("", "supertonic-*.wav")
	if err != nil {
		log.Printf("Error creating temp file: %v", err)
		return nil
	}
	defer os.Remove(tmpfile.Name())
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

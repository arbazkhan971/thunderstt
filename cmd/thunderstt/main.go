package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/arbaz/thunderstt/internal/api"
	"github.com/arbaz/thunderstt/internal/audio"
	"github.com/arbaz/thunderstt/internal/config"
	"github.com/arbaz/thunderstt/internal/engine"
	"github.com/arbaz/thunderstt/internal/format"
	"github.com/arbaz/thunderstt/internal/model"
	"github.com/arbaz/thunderstt/internal/pipeline"
)

var (
	// Version information set via ldflags at build time.
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// newRootCmd builds the top-level cobra command and attaches all sub-commands.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "thunderstt",
		Short: "ThunderSTT -- high-performance speech-to-text server",
		Long: `ThunderSTT is a speech-to-text server that exposes transcription
capabilities over HTTP. It supports multiple models, concurrent workers,
Prometheus metrics, and structured JSON logging.`,
		SilenceUsage: true,
	}

	root.AddCommand(
		newServeCmd(),
		newTranscribeCmd(),
		newDownloadCmd(),
		newModelsCmd(),
		newVersionCmd(),
	)

	return root
}

// ---------------------------------------------------------------------------
// serve
// ---------------------------------------------------------------------------

func newServeCmd() *cobra.Command {
	var (
		host     string
		port     int
		model    string
		workers  int
		logLevel string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP transcription server",
		Long: `Start the ThunderSTT HTTP server. The server exposes a REST API for
submitting audio files and retrieving transcription results. Configuration
can also be provided through environment variables (THUNDERSTT_*).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.NewFromServeFlags(host, port, model, workers, logLevel)
			setupLogging(cfg.LogLevel)

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			log.Info().
				Str("host", cfg.Host).
				Int("port", cfg.Port).
				Str("model", cfg.Model).
				Int("workers", cfg.Workers).
				Msg("starting server")

			// Use a NoopEngine as a placeholder since we cannot load sherpa
			// without CGo. Replace with a real engine when CGo is available.
			eng := engine.NewNoopEngine(cfg.Model)

			p := pipeline.New(eng)
			defer p.Close()

			srv := api.NewServer(p, cfg)
			return srv.Start()
		},
	}

	cmd.Flags().StringVar(&host, "host", "0.0.0.0", "address to bind the HTTP server to")
	cmd.Flags().IntVar(&port, "port", 8080, "port to listen on")
	cmd.Flags().StringVar(&model, "model", "base", "whisper model to load (tiny, base, small, medium, large)")
	cmd.Flags().IntVar(&workers, "workers", runtime.NumCPU(), "number of concurrent transcription workers")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "log level (trace, debug, info, warn, error, fatal)")

	return cmd
}

// ---------------------------------------------------------------------------
// transcribe
// ---------------------------------------------------------------------------

func newTranscribeCmd() *cobra.Command {
	var (
		model          string
		outputFormat   string
		wordTimestamps bool
		language       string
	)

	cmd := &cobra.Command{
		Use:   "transcribe [file]",
		Short: "Transcribe an audio file locally",
		Long: `Transcribe a single audio file using a locally-loaded model. The result
is printed to stdout in the requested format.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			setupLogging("info")

			if !format.IsValidFormat(outputFormat) {
				return fmt.Errorf("unsupported output format %q; valid formats: text, json, srt, vtt", outputFormat)
			}

			log.Info().
				Str("file", filePath).
				Str("model", model).
				Str("format", outputFormat).
				Bool("word_timestamps", wordTimestamps).
				Str("language", language).
				Msg("transcribing")

			// Verify the audio file is readable before proceeding.
			if _, err := os.Stat(filePath); err != nil {
				return fmt.Errorf("cannot access audio file: %w", err)
			}

			// Use a NoopEngine as a placeholder (no CGo required).
			eng := engine.NewNoopEngine(model)

			pipelineCfg := pipeline.PipelineConfig{
				WordTimestamps: wordTimestamps,
				Language:       language,
			}
			p, err := pipeline.NewPipeline(eng, pipelineCfg)
			if err != nil {
				return fmt.Errorf("failed to create pipeline: %w", err)
			}
			defer p.Close()

			// Decode the audio file.
			samples, sampleRate, err := audio.DecodeFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to decode audio: %w", err)
			}

			// Run transcription.
			result, err := p.ProcessAudio(samples, sampleRate)
			if err != nil {
				return fmt.Errorf("transcription failed: %w", err)
			}

			// Format and print the output.
			output, _, err := format.FormatResult(result, outputFormat)
			if err != nil {
				return fmt.Errorf("failed to format result: %w", err)
			}

			fmt.Print(string(output))
			return nil
		},
	}

	cmd.Flags().StringVar(&model, "model", "base", "whisper model to use")
	cmd.Flags().StringVar(&outputFormat, "format", "text", "output format (text, json, srt, vtt)")
	cmd.Flags().BoolVar(&wordTimestamps, "word-timestamps", false, "include word-level timestamps")
	cmd.Flags().StringVar(&language, "language", "", "language code (e.g. en, es, fr); empty for auto-detect")

	return cmd
}

// ---------------------------------------------------------------------------
// download
// ---------------------------------------------------------------------------

func newDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download [model]",
		Short: "Download a whisper model",
		Long: `Download the specified whisper model to the local models directory.
The models directory defaults to ~/.thunderstt/models and can be overridden
with the THUNDERSTT_MODELS_DIR environment variable.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]
			cfg := config.NewFromEnv()
			setupLogging("info")

			log.Info().
				Str("model", modelName).
				Str("models_dir", cfg.ModelsDir).
				Msg("downloading model...")

			fmt.Printf("downloading model=%s to %s\n", modelName, cfg.ModelsDir)
			return nil
		},
	}

	return cmd
}

// ---------------------------------------------------------------------------
// models
// ---------------------------------------------------------------------------

func newModelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List available whisper models",
		Run: func(cmd *cobra.Command, args []string) {
			models := model.ListModels()

			fmt.Println("Available models:")
			fmt.Println()
			fmt.Printf("  %-28s %-12s %-10s %s\n", "ID", "ENGINE", "SIZE", "TYPE")
			fmt.Printf("  %-28s %-12s %-10s %s\n", "---", "------", "----", "----")
			for _, m := range models {
				fmt.Printf("  %-28s %-12s %-10s %s\n", m.ID, m.Engine, m.Size, m.ModelType)
			}
		},
	}

	return cmd
}

// ---------------------------------------------------------------------------
// version
// ---------------------------------------------------------------------------

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("thunderstt %s\n", Version)
			fmt.Printf("  commit:     %s\n", Commit)
			fmt.Printf("  built:      %s\n", BuildDate)
			fmt.Printf("  go version: %s\n", runtime.Version())
			fmt.Printf("  os/arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}

	return cmd
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// setupLogging configures zerolog with the given level string.
func setupLogging(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Caller().
		Logger()
}

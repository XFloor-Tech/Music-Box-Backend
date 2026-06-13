package converter

import (
	"context"
	"time"
)

const (
	DefaultFFmpegPath  = "ffmpeg"
	DefaultFFprobePath = "ffprobe"
	DefaultMP4BoxPath  = "MP4Box"

	DefaultAACBitrate           = "256k"
	DefaultDashDurationMs       = 10000
	DefaultFragmentDurationMs   = 2000
	DefaultDashProfile          = "live"
	DefaultDashSegmentName      = "$Init=init$:$Segment=$Number$.m4s"
	DefaultManifestFileName     = "manifest.mpd"
	DefaultInitFileName         = "init.mp4"
	DefaultIntermediateFileName = "audio.mp4"

	DefaultDurationTolerance = 2 * time.Second
)

type CommandRunner interface {
	Run(context.Context, Command) ([]byte, error)
}

type Command struct {
	Dir  string
	Name string
	Args []string
}

type ProcessDashAudioOptions struct {
	FFmpegPath  string
	FFprobePath string
	MP4BoxPath  string
	Runner      CommandRunner

	InputName            string
	TempDir              string
	AACBitrate           string
	DashDurationMs       int
	FragmentDurationMs   int
	DashProfile          string
	DashSegmentName      string
	ManifestFileName     string
	InitFileName         string
	IntermediateFileName string
	KeepIntermediate     bool
	DurationTolerance    time.Duration
}

type AACOptions struct {
	FFmpegPath        string
	FFprobePath       string
	Runner            CommandRunner
	InputName         string
	TempDir           string
	Bitrate           string
	DurationTolerance time.Duration
}

type VerifyAudioOptions struct {
	FFprobePath            string
	Runner                 CommandRunner
	ExpectedCodec          string
	ExpectedFormatContains string
	RequireNoVideo         bool
	MinDuration            time.Duration
}

type AudioInfo struct {
	Path         string
	Format       string
	Codec        string
	Duration     time.Duration
	SampleRateHz int
	ChannelCount int
	BitrateBps   int
	SizeBytes    int64
	AudioStreams int
	VideoStreams int
}

type DashAudioResult struct {
	Source       AudioInfo
	AAC          AudioInfo
	AACPath      string
	ManifestPath string
	InitPath     string
	Segments     []DashSegment
	Artifacts    []DashArtifact
}

type DashArtifactKind string

const (
	DashArtifactManifest DashArtifactKind = "manifest"
	DashArtifactInit     DashArtifactKind = "init"
	DashArtifactSegment  DashArtifactKind = "segment"
	DashArtifactAAC      DashArtifactKind = "aac"
)

type DashArtifact struct {
	Kind      DashArtifactKind
	Name      string
	Path      string
	SizeBytes int64
}

type DashSegment struct {
	Part          int64
	Time          int64
	DurationUnits int64
	Name          string
	Path          string
	SizeBytes     int64
}

type dashConfig struct {
	ffmpegPath           string
	ffprobePath          string
	mp4boxPath           string
	runner               CommandRunner
	inputName            string
	tempDir              string
	aacBitrate           string
	dashDurationMs       int
	fragmentDurationMs   int
	dashProfile          string
	dashSegmentName      string
	manifestFileName     string
	initFileName         string
	intermediateFileName string
	keepIntermediate     bool
	durationTolerance    time.Duration
}

type aacConfig struct {
	ffmpegPath        string
	ffprobePath       string
	runner            CommandRunner
	inputName         string
	tempDir           string
	bitrate           string
	durationTolerance time.Duration
}

func normalizeDashOptions(options ProcessDashAudioOptions) dashConfig {
	cfg := dashConfig{
		ffmpegPath:           valueOrDefault(options.FFmpegPath, DefaultFFmpegPath),
		ffprobePath:          valueOrDefault(options.FFprobePath, DefaultFFprobePath),
		mp4boxPath:           valueOrDefault(options.MP4BoxPath, DefaultMP4BoxPath),
		runner:               options.Runner,
		inputName:            options.InputName,
		tempDir:              options.TempDir,
		aacBitrate:           valueOrDefault(options.AACBitrate, DefaultAACBitrate),
		dashDurationMs:       options.DashDurationMs,
		fragmentDurationMs:   options.FragmentDurationMs,
		dashProfile:          valueOrDefault(options.DashProfile, DefaultDashProfile),
		dashSegmentName:      valueOrDefault(options.DashSegmentName, DefaultDashSegmentName),
		manifestFileName:     valueOrDefault(options.ManifestFileName, DefaultManifestFileName),
		initFileName:         valueOrDefault(options.InitFileName, DefaultInitFileName),
		intermediateFileName: valueOrDefault(options.IntermediateFileName, DefaultIntermediateFileName),
		keepIntermediate:     options.KeepIntermediate,
		durationTolerance:    options.DurationTolerance,
	}
	if cfg.runner == nil {
		cfg.runner = execCommandRunner{}
	}
	if cfg.dashDurationMs < 1 {
		cfg.dashDurationMs = DefaultDashDurationMs
	}
	if cfg.fragmentDurationMs < 1 {
		cfg.fragmentDurationMs = DefaultFragmentDurationMs
	}
	if cfg.durationTolerance <= 0 {
		cfg.durationTolerance = DefaultDurationTolerance
	}

	return cfg
}

func normalizeAACOptions(options AACOptions) aacConfig {
	cfg := aacConfig{
		ffmpegPath:        valueOrDefault(options.FFmpegPath, DefaultFFmpegPath),
		ffprobePath:       valueOrDefault(options.FFprobePath, DefaultFFprobePath),
		runner:            options.Runner,
		inputName:         options.InputName,
		tempDir:           options.TempDir,
		bitrate:           valueOrDefault(options.Bitrate, DefaultAACBitrate),
		durationTolerance: options.DurationTolerance,
	}
	if cfg.runner == nil {
		cfg.runner = execCommandRunner{}
	}
	if cfg.durationTolerance <= 0 {
		cfg.durationTolerance = DefaultDurationTolerance
	}

	return cfg
}

func normalizeVerifyOptions(options VerifyAudioOptions) VerifyAudioOptions {
	options.FFprobePath = valueOrDefault(options.FFprobePath, DefaultFFprobePath)
	if options.Runner == nil {
		options.Runner = execCommandRunner{}
	}

	return options
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

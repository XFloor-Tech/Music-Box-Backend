package converter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func ProcessDashAudio(ctx context.Context, input io.Reader, outputDir string, options ProcessDashAudioOptions) (DashAudioResult, error) {
	return processDashAudio(ctx, input, outputDir, options)
}

func ProcessDashAudioFile(ctx context.Context, inputPath string, outputDir string, options ProcessDashAudioOptions) (DashAudioResult, error) {
	cfg := normalizeDashOptions(options)
	return processDashAudioFile(ctx, strings.TrimSpace(inputPath), strings.TrimSpace(outputDir), cfg)
}

func ToAAC(ctx context.Context, input io.Reader, outputPath string, options AACOptions) (AudioInfo, error) {
	return toAAC(ctx, input, outputPath, options)
}

func ToAACFile(ctx context.Context, inputPath string, outputPath string, options AACOptions) (AudioInfo, error) {
	cfg := normalizeAACOptions(options)
	return toAACFile(ctx, strings.TrimSpace(inputPath), strings.TrimSpace(outputPath), cfg)
}

func VerifyAudioFile(ctx context.Context, path string, options VerifyAudioOptions) (AudioInfo, error) {
	options = normalizeVerifyOptions(options)
	path = strings.TrimSpace(path)
	if path == "" {
		return AudioInfo{}, fmt.Errorf("audio path is required")
	}

	stat, err := os.Stat(path)
	if err != nil {
		return AudioInfo{}, fmt.Errorf("stat audio file: %w", err)
	}
	if stat.IsDir() {
		return AudioInfo{}, fmt.Errorf("audio path must be a file")
	}
	if stat.Size() == 0 {
		return AudioInfo{}, fmt.Errorf("audio file is empty")
	}

	info, err := probeAudioFile(ctx, path, options.FFprobePath, options.Runner)
	if err != nil {
		return AudioInfo{}, err
	}
	info.SizeBytes = stat.Size()

	if info.AudioStreams == 0 || strings.TrimSpace(info.Codec) == "" {
		return AudioInfo{}, fmt.Errorf("audio file has no audio stream")
	}
	if options.ExpectedCodec != "" && !strings.EqualFold(info.Codec, options.ExpectedCodec) {
		return AudioInfo{}, fmt.Errorf("audio codec mismatch: got %q, want %q", info.Codec, options.ExpectedCodec)
	}
	if options.ExpectedFormatContains != "" && !strings.Contains(strings.ToLower(info.Format), strings.ToLower(options.ExpectedFormatContains)) {
		return AudioInfo{}, fmt.Errorf("audio format mismatch: got %q, want format containing %q", info.Format, options.ExpectedFormatContains)
	}
	if options.RequireNoVideo && info.VideoStreams > 0 {
		return AudioInfo{}, fmt.Errorf("audio file contains video stream")
	}
	if options.MinDuration > 0 && info.Duration < options.MinDuration {
		return AudioInfo{}, fmt.Errorf("audio duration is too short: got %s, want at least %s", info.Duration, options.MinDuration)
	}

	return info, nil
}

func processDashAudio(ctx context.Context, input io.Reader, outputDir string, options ProcessDashAudioOptions) (DashAudioResult, error) {
	if input == nil {
		return DashAudioResult{}, fmt.Errorf("audio input is required")
	}

	cfg := normalizeDashOptions(options)
	tempDir, cleanup, err := converterTempDir(cfg.tempDir)
	if err != nil {
		return DashAudioResult{}, err
	}
	defer cleanup()

	inputPath := filepath.Join(tempDir, "input"+sourceExtension(cfg.inputName))
	if err := writeReaderToFile(inputPath, input); err != nil {
		return DashAudioResult{}, err
	}

	return processDashAudioFile(ctx, inputPath, strings.TrimSpace(outputDir), cfg)
}

func processDashAudioFile(ctx context.Context, inputPath string, outputDir string, cfg dashConfig) (DashAudioResult, error) {
	if inputPath == "" {
		return DashAudioResult{}, fmt.Errorf("audio input path is required")
	}
	if outputDir == "" {
		return DashAudioResult{}, fmt.Errorf("dash output directory is required")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return DashAudioResult{}, fmt.Errorf("create dash output directory: %w", err)
	}

	source, err := VerifyAudioFile(ctx, inputPath, VerifyAudioOptions{
		FFprobePath: cfg.ffprobePath,
		Runner:      cfg.runner,
	})
	if err != nil {
		return DashAudioResult{}, fmt.Errorf("verify source audio: %w", err)
	}

	aacPath := filepath.Join(outputDir, cfg.intermediateFileName)
	if !cfg.keepIntermediate {
		tempDir, cleanup, err := converterTempDir(cfg.tempDir)
		if err != nil {
			return DashAudioResult{}, err
		}
		defer cleanup()
		aacPath = filepath.Join(tempDir, cfg.intermediateFileName)
	}

	aac, err := toAACFile(ctx, inputPath, aacPath, aacConfig{
		ffmpegPath:        cfg.ffmpegPath,
		ffprobePath:       cfg.ffprobePath,
		runner:            cfg.runner,
		bitrate:           cfg.aacBitrate,
		durationTolerance: cfg.durationTolerance,
	})
	if err != nil {
		return DashAudioResult{}, err
	}

	if err := verifyTranscodeMatch(source, aac, cfg.durationTolerance); err != nil {
		return DashAudioResult{}, err
	}

	if err := runMP4BoxDash(ctx, aacPath, outputDir, cfg); err != nil {
		return DashAudioResult{}, err
	}

	manifestPath := filepath.Join(outputDir, cfg.manifestFileName)
	packageInfo, err := VerifyDashPackage(outputDir, manifestPath, VerifyDashPackageOptions{
		InitFileName: cfg.initFileName,
	})
	if err != nil {
		return DashAudioResult{}, fmt.Errorf("verify dash package: %w", err)
	}

	result := DashAudioResult{
		Source:       source,
		AAC:          aac,
		ManifestPath: packageInfo.ManifestPath,
		InitPath:     packageInfo.InitPath,
		Segments:     packageInfo.Segments,
		Artifacts:    packageInfo.Artifacts,
	}

	if cfg.keepIntermediate {
		result.AACPath = aacPath
		aacStat, err := os.Stat(aacPath)
		if err != nil {
			return DashAudioResult{}, fmt.Errorf("stat intermediate aac file: %w", err)
		}
		result.Artifacts = append(result.Artifacts, DashArtifact{
			Kind:      DashArtifactAAC,
			Name:      filepath.Base(aacPath),
			Path:      aacPath,
			SizeBytes: aacStat.Size(),
		})
	}

	return result, nil
}

func toAAC(ctx context.Context, input io.Reader, outputPath string, options AACOptions) (AudioInfo, error) {
	if input == nil {
		return AudioInfo{}, fmt.Errorf("audio input is required")
	}

	cfg := normalizeAACOptions(options)
	tempDir, cleanup, err := converterTempDir(cfg.tempDir)
	if err != nil {
		return AudioInfo{}, err
	}
	defer cleanup()

	inputPath := filepath.Join(tempDir, "input"+sourceExtension(cfg.inputName))
	if err := writeReaderToFile(inputPath, input); err != nil {
		return AudioInfo{}, err
	}

	return toAACFile(ctx, inputPath, strings.TrimSpace(outputPath), cfg)
}

func toAACFile(ctx context.Context, inputPath string, outputPath string, cfg aacConfig) (AudioInfo, error) {
	if inputPath == "" {
		return AudioInfo{}, fmt.Errorf("audio input path is required")
	}
	if outputPath == "" {
		return AudioInfo{}, fmt.Errorf("aac output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return AudioInfo{}, fmt.Errorf("create aac output directory: %w", err)
	}

	source, err := VerifyAudioFile(ctx, inputPath, VerifyAudioOptions{
		FFprobePath: cfg.ffprobePath,
		Runner:      cfg.runner,
	})
	if err != nil {
		return AudioInfo{}, fmt.Errorf("verify source audio: %w", err)
	}

	args := []string{
		"-hide_banner",
		"-nostdin",
		"-y",
		"-i", inputPath,
		"-vn",
		"-map", "0:a:0",
		"-c:a", "aac",
		"-b:a", cfg.bitrate,
		outputPath,
	}
	if _, err := cfg.runner.Run(ctx, Command{Name: cfg.ffmpegPath, Args: args}); err != nil {
		return AudioInfo{}, fmt.Errorf("convert audio to aac: %w", err)
	}

	aac, err := VerifyAudioFile(ctx, outputPath, VerifyAudioOptions{
		FFprobePath:            cfg.ffprobePath,
		Runner:                 cfg.runner,
		ExpectedCodec:          "aac",
		ExpectedFormatContains: "mp4",
		RequireNoVideo:         true,
	})
	if err != nil {
		return AudioInfo{}, fmt.Errorf("verify aac output: %w", err)
	}

	if err := verifyTranscodeMatch(source, aac, cfg.durationTolerance); err != nil {
		return AudioInfo{}, err
	}

	return aac, nil
}

func runMP4BoxDash(ctx context.Context, aacPath string, outputDir string, cfg dashConfig) error {
	args := []string{
		"-dash", strconv.Itoa(cfg.dashDurationMs),
		"-frag", strconv.Itoa(cfg.fragmentDurationMs),
		"-rap",
		"-profile", cfg.dashProfile,
		"-segment-timeline",
		"-url-template",
		"-segment-name", cfg.dashSegmentName,
		"-out", cfg.manifestFileName,
		aacPath,
	}
	if _, err := cfg.runner.Run(ctx, Command{Dir: outputDir, Name: cfg.mp4boxPath, Args: args}); err != nil {
		return fmt.Errorf("package dash audio: %w", err)
	}

	return nil
}

func probeAudioFile(ctx context.Context, path string, ffprobePath string, runner CommandRunner) (AudioInfo, error) {
	output, err := runner.Run(ctx, Command{
		Name: ffprobePath,
		Args: []string{
			"-v", "error",
			"-print_format", "json",
			"-show_format",
			"-show_streams",
			path,
		},
	})
	if err != nil {
		return AudioInfo{}, fmt.Errorf("probe audio file: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return AudioInfo{}, fmt.Errorf("parse ffprobe output: %w", err)
	}

	info := AudioInfo{
		Path:   path,
		Format: strings.TrimSpace(probe.Format.FormatName),
	}

	if duration, ok := parseSeconds(probe.Format.Duration); ok {
		info.Duration = duration
	}
	info.BitrateBps = parseInt(probe.Format.BitRate)

	for _, stream := range probe.Streams {
		switch strings.ToLower(strings.TrimSpace(stream.CodecType)) {
		case "audio":
			info.AudioStreams++
			if info.Codec == "" {
				info.Codec = strings.TrimSpace(stream.CodecName)
				info.SampleRateHz = parseInt(stream.SampleRate)
				info.ChannelCount = stream.Channels
				if duration, ok := parseSeconds(stream.Duration); ok && duration > info.Duration {
					info.Duration = duration
				}
				if bitrate := parseInt(stream.BitRate); bitrate > 0 {
					info.BitrateBps = bitrate
				}
			}
		case "video":
			info.VideoStreams++
		}
	}

	return info, nil
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecName  string `json:"codec_name"`
	CodecType  string `json:"codec_type"`
	SampleRate string `json:"sample_rate"`
	Channels   int    `json:"channels"`
	Duration   string `json:"duration"`
	BitRate    string `json:"bit_rate"`
}

type ffprobeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	BitRate    string `json:"bit_rate"`
}

func verifyTranscodeMatch(source AudioInfo, output AudioInfo, tolerance time.Duration) error {
	if source.SampleRateHz > 0 && output.SampleRateHz > 0 && source.SampleRateHz != output.SampleRateHz {
		return fmt.Errorf("aac sample rate mismatch: got %d, want %d", output.SampleRateHz, source.SampleRateHz)
	}
	if source.ChannelCount > 0 && output.ChannelCount > 0 && source.ChannelCount != output.ChannelCount {
		return fmt.Errorf("aac channel count mismatch: got %d, want %d", output.ChannelCount, source.ChannelCount)
	}
	if source.Duration > 0 && output.Duration > 0 {
		diff := source.Duration - output.Duration
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			return fmt.Errorf("aac duration mismatch: got %s, want %s within %s", output.Duration, source.Duration, tolerance)
		}
	}

	return nil
}

func converterTempDir(parent string) (string, func(), error) {
	tempDir, err := os.MkdirTemp(strings.TrimSpace(parent), "music-box-converter-*")
	if err != nil {
		return "", nil, fmt.Errorf("create converter temp directory: %w", err)
	}

	return tempDir, func() {
		_ = os.RemoveAll(tempDir)
	}, nil
}

func writeReaderToFile(path string, input io.Reader) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create input file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, input); err != nil {
		return fmt.Errorf("write input file: %w", err)
	}

	return nil
}

func sourceExtension(name string) string {
	extension := filepath.Ext(strings.TrimSpace(name))
	if extension == "" || strings.Contains(extension, string(filepath.Separator)) {
		return ""
	}

	return extension
}

func parseSeconds(value string) (time.Duration, bool) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, false
	}

	return time.Duration(seconds * float64(time.Second)), true
}

func parseInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0
	}

	return parsed
}

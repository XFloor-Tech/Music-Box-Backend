package converter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestToAACFileRunsFFmpegAndVerifiesAACOutput(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.wav")
	outputPath := filepath.Join(dir, "output.mp4")
	writeTestFile(t, inputPath, "source")

	runner := &fakeCommandRunner{
		probeOutput: func(path string) string {
			switch path {
			case inputPath:
				return probeJSON("wav", "pcm_s16le", "12.000000", 44100, 2, false)
			case outputPath:
				return probeJSON("mov,mp4,m4a,3gp,3g2,mj2", "aac", "12.010000", 44100, 2, false)
			default:
				t.Fatalf("unexpected probe path %q", path)
				return ""
			}
		},
	}

	info, err := ToAACFile(context.Background(), inputPath, outputPath, AACOptions{
		Runner: runner,
	})
	if err != nil {
		t.Fatalf("ToAACFile returned error: %v", err)
	}

	if info.Codec != "aac" {
		t.Fatalf("codec = %q, want aac", info.Codec)
	}

	ffmpeg := runner.commandByName(DefaultFFmpegPath)
	if ffmpeg == nil {
		t.Fatal("ffmpeg was not called")
	}

	wantArgs := []string{"-i", inputPath, "-vn", "-map", "0:a:0", "-c:a", "aac", "-b:a", "256k", outputPath}
	for _, arg := range wantArgs {
		if !containsArg(ffmpeg.Args, arg) {
			t.Fatalf("ffmpeg args %v missing %q", ffmpeg.Args, arg)
		}
	}
}

func TestToAACFileRejectsCodecMismatch(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.wav")
	outputPath := filepath.Join(dir, "output.mp4")
	writeTestFile(t, inputPath, "source")

	runner := &fakeCommandRunner{
		probeOutput: func(path string) string {
			if path == inputPath {
				return probeJSON("wav", "pcm_s16le", "12.000000", 44100, 2, false)
			}

			return probeJSON("mov,mp4,m4a,3gp,3g2,mj2", "mp3", "12.000000", 44100, 2, false)
		},
	}

	_, err := ToAACFile(context.Background(), inputPath, outputPath, AACOptions{
		Runner: runner,
	})
	if err == nil {
		t.Fatal("ToAACFile returned nil error")
	}
	if !strings.Contains(err.Error(), "audio codec mismatch") {
		t.Fatalf("error = %q, want codec mismatch", err.Error())
	}
}

func TestProcessDashAudioUsesConceptOptionsAndReturnsVerifiedArtifacts(t *testing.T) {
	dir := t.TempDir()

	runner := &fakeCommandRunner{
		probeOutput: func(path string) string {
			if filepath.Base(path) == DefaultIntermediateFileName {
				return probeJSON("mov,mp4,m4a,3gp,3g2,mj2", "aac", "30.000000", 44100, 2, false)
			}

			return probeJSON("mp3", "mp3", "30.000000", 44100, 2, false)
		},
		onMP4Box: func(command Command) error {
			writeTestFile(t, filepath.Join(command.Dir, DefaultManifestFileName), `<?xml version="1.0"?>
<MPD>
  <Period>
    <AdaptationSet mimeType="audio/mp4">
      <Representation id="1">
        <SegmentTemplate media="$Time$.m4s" initialization="init.mp4" timescale="44100" startNumber="1">
          <SegmentTimeline>
            <S t="0" d="440320" />
            <S d="441344" />
          </SegmentTimeline>
        </SegmentTemplate>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`)
			writeTestFile(t, filepath.Join(command.Dir, DefaultInitFileName), "init")
			writeTestFile(t, filepath.Join(command.Dir, "0.m4s"), "segment-1")
			writeTestFile(t, filepath.Join(command.Dir, "440320.m4s"), "segment-2")
			return nil
		},
	}

	result, err := ProcessDashAudio(context.Background(), strings.NewReader("audio bytes"), dir, ProcessDashAudioOptions{
		Runner:    runner,
		InputName: "song.mp3",
	})
	if err != nil {
		t.Fatalf("ProcessDashAudio returned error: %v", err)
	}

	if result.ManifestPath != filepath.Join(dir, DefaultManifestFileName) {
		t.Fatalf("manifest path = %q, want output manifest", result.ManifestPath)
	}
	if result.InitPath != filepath.Join(dir, DefaultInitFileName) {
		t.Fatalf("init path = %q, want output init", result.InitPath)
	}
	if len(result.Segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(result.Segments))
	}
	if result.Segments[1].Name != "440320.m4s" {
		t.Fatalf("second segment = %q, want timeline segment", result.Segments[1].Name)
	}

	mp4box := runner.commandByName(DefaultMP4BoxPath)
	if mp4box == nil {
		t.Fatal("MP4Box was not called")
	}

	wantArgs := []string{
		"-dash", "10000",
		"-frag", "2000",
		"-rap",
		"-profile", "live",
		"-segment-timeline",
		"-url-template",
		"-segment-name", "$Init=init$:$Segment=$Number$.m4s",
		"-out", "manifest.mpd",
	}
	for _, arg := range wantArgs {
		if !containsArg(mp4box.Args, arg) {
			t.Fatalf("MP4Box args %v missing %q", mp4box.Args, arg)
		}
	}
}

func TestVerifyDashPackageRejectsMissingTimelineSegment(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, DefaultManifestFileName)
	writeTestFile(t, manifestPath, `<?xml version="1.0"?>
<MPD>
  <Period>
    <AdaptationSet>
      <Representation>
        <SegmentTemplate media="$Time$.m4s" initialization="init.mp4" timescale="44100" startNumber="1">
          <SegmentTimeline>
            <S t="0" d="440320" />
          </SegmentTimeline>
        </SegmentTemplate>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`)
	writeTestFile(t, filepath.Join(dir, DefaultInitFileName), "init")

	_, err := VerifyDashPackage(dir, manifestPath, VerifyDashPackageOptions{})
	if err == nil {
		t.Fatal("VerifyDashPackage returned nil error")
	}
	if !strings.Contains(err.Error(), "0.m4s") {
		t.Fatalf("error = %q, want missing segment name", err.Error())
	}
}

type fakeCommandRunner struct {
	commands    []Command
	probeOutput func(path string) string
	onMP4Box    func(Command) error
}

func (r *fakeCommandRunner) Run(_ context.Context, command Command) ([]byte, error) {
	r.commands = append(r.commands, command)

	switch command.Name {
	case DefaultFFprobePath:
		if r.probeOutput == nil {
			return nil, errors.New("probe output is not configured")
		}

		return []byte(r.probeOutput(command.Args[len(command.Args)-1])), nil
	case DefaultFFmpegPath:
		writeTestFile(nil, command.Args[len(command.Args)-1], "aac")
		return nil, nil
	case DefaultMP4BoxPath:
		if r.onMP4Box != nil {
			if err := r.onMP4Box(command); err != nil {
				return nil, err
			}
		}
		return nil, nil
	default:
		return nil, errors.New("unexpected command: " + command.Name)
	}
}

func (r *fakeCommandRunner) commandByName(name string) *Command {
	for i := range r.commands {
		if r.commands[i].Name == name {
			return &r.commands[i]
		}
	}

	return nil
}

func probeJSON(format string, codec string, duration string, sampleRate int, channels int, hasVideo bool) string {
	videoStream := ""
	if hasVideo {
		videoStream = `,{"codec_name":"h264","codec_type":"video"}`
	}

	return `{
  "streams": [
    {
      "codec_name": "` + codec + `",
      "codec_type": "audio",
      "sample_rate": "` + strconvItoa(sampleRate) + `",
      "channels": ` + strconvItoa(channels) + `,
      "duration": "` + duration + `",
      "bit_rate": "256000"
    }` + videoStream + `
  ],
  "format": {
    "format_name": "` + format + `",
    "duration": "` + duration + `",
    "bit_rate": "256000"
  }
}`
}

func writeTestFile(t *testing.T, path string, contents string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		if t != nil {
			t.Fatalf("mkdir: %v", err)
		}
		panic(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		if t != nil {
			t.Fatalf("write file: %v", err)
		}
		panic(err)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}

	return false
}

func strconvItoa(value int) string {
	return strconv.FormatInt(int64(value), 10)
}

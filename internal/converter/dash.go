package converter

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type VerifyDashPackageOptions struct {
	InitFileName string
}

type DashPackageInfo struct {
	ManifestPath string
	InitPath     string
	Segments     []DashSegment
	Artifacts    []DashArtifact
}

type dashSegmentTemplate struct {
	Media          string              `xml:"media,attr"`
	Initialization string              `xml:"initialization,attr"`
	Timescale      int64               `xml:"timescale,attr"`
	StartNumber    int64               `xml:"startNumber,attr"`
	Timeline       dashSegmentTimeline `xml:"SegmentTimeline"`
}

type dashSegmentTimeline struct {
	Entries []dashTimelineEntry `xml:"S"`
}

type dashTimelineEntry struct {
	T *int64 `xml:"t,attr"`
	D int64  `xml:"d,attr"`
	R *int64 `xml:"r,attr"`
}

func VerifyDashPackage(outputDir string, manifestPath string, options VerifyDashPackageOptions) (DashPackageInfo, error) {
	outputDir = strings.TrimSpace(outputDir)
	manifestPath = strings.TrimSpace(manifestPath)
	if outputDir == "" {
		return DashPackageInfo{}, fmt.Errorf("dash output directory is required")
	}
	if manifestPath == "" {
		return DashPackageInfo{}, fmt.Errorf("dash manifest path is required")
	}

	manifestStat, err := nonEmptyFile(manifestPath)
	if err != nil {
		return DashPackageInfo{}, fmt.Errorf("manifest.mpd is invalid: %w", err)
	}

	template, err := parseSegmentTemplate(manifestPath)
	if err != nil {
		return DashPackageInfo{}, err
	}
	if strings.TrimSpace(template.Media) == "" {
		return DashPackageInfo{}, fmt.Errorf("dash manifest SegmentTemplate media is required")
	}
	if strings.TrimSpace(template.Initialization) == "" {
		return DashPackageInfo{}, fmt.Errorf("dash manifest SegmentTemplate initialization is required")
	}
	if len(template.Timeline.Entries) == 0 {
		return DashPackageInfo{}, fmt.Errorf("dash manifest SegmentTimeline is required")
	}

	expectedInit := valueOrDefault(options.InitFileName, DefaultInitFileName)
	if filepath.Base(template.Initialization) != expectedInit {
		return DashPackageInfo{}, fmt.Errorf("dash init mismatch: got %q, want %q", template.Initialization, expectedInit)
	}

	initPath, err := safeArtifactPath(outputDir, template.Initialization)
	if err != nil {
		return DashPackageInfo{}, err
	}
	initStat, err := nonEmptyFile(initPath)
	if err != nil {
		return DashPackageInfo{}, fmt.Errorf("init segment is invalid: %w", err)
	}

	segments, err := expandDashSegments(outputDir, template)
	if err != nil {
		return DashPackageInfo{}, err
	}
	if len(segments) == 0 {
		return DashPackageInfo{}, fmt.Errorf("dash manifest has no media segments")
	}

	artifacts := []DashArtifact{
		{
			Kind:      DashArtifactManifest,
			Name:      filepath.Base(manifestPath),
			Path:      manifestPath,
			SizeBytes: manifestStat.Size(),
		},
		{
			Kind:      DashArtifactInit,
			Name:      filepath.Base(initPath),
			Path:      initPath,
			SizeBytes: initStat.Size(),
		},
	}
	for _, segment := range segments {
		artifacts = append(artifacts, DashArtifact{
			Kind:      DashArtifactSegment,
			Name:      segment.Name,
			Path:      segment.Path,
			SizeBytes: segment.SizeBytes,
		})
	}

	return DashPackageInfo{
		ManifestPath: manifestPath,
		InitPath:     initPath,
		Segments:     segments,
		Artifacts:    artifacts,
	}, nil
}

func parseSegmentTemplate(manifestPath string) (dashSegmentTemplate, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return dashSegmentTemplate{}, fmt.Errorf("open dash manifest: %w", err)
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}

			return dashSegmentTemplate{}, fmt.Errorf("parse dash manifest: %w", err)
		}

		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "SegmentTemplate" {
			continue
		}

		var template dashSegmentTemplate
		if err := decoder.DecodeElement(&template, &start); err != nil {
			return dashSegmentTemplate{}, fmt.Errorf("parse dash SegmentTemplate: %w", err)
		}
		if template.StartNumber == 0 {
			template.StartNumber = 1
		}

		return template, nil
	}

	return dashSegmentTemplate{}, fmt.Errorf("dash manifest SegmentTemplate is required")
}

func expandDashSegments(outputDir string, template dashSegmentTemplate) ([]DashSegment, error) {
	part := template.StartNumber
	currentTime := int64(0)
	segments := []DashSegment{}

	for _, entry := range template.Timeline.Entries {
		if entry.D <= 0 {
			return nil, fmt.Errorf("dash timeline duration must be greater than 0")
		}
		if entry.T != nil {
			currentTime = *entry.T
		}

		repeat := int64(0)
		if entry.R != nil {
			repeat = *entry.R
		}
		if repeat < 0 {
			return nil, fmt.Errorf("dash timeline negative repeat is not supported")
		}

		for i := int64(0); i <= repeat; i++ {
			segmentName, err := resolveDashTemplate(template.Media, part, currentTime)
			if err != nil {
				return nil, err
			}
			segmentPath, err := safeArtifactPath(outputDir, segmentName)
			if err != nil {
				return nil, err
			}
			segmentStat, err := nonEmptyFile(segmentPath)
			if err != nil {
				return nil, fmt.Errorf("media segment %q is invalid: %w", segmentName, err)
			}
			segmentName = filepath.Clean(segmentName)

			segments = append(segments, DashSegment{
				Part:          part,
				Time:          currentTime,
				DurationUnits: entry.D,
				Name:          segmentName,
				Path:          segmentPath,
				SizeBytes:     segmentStat.Size(),
			})
			currentTime += entry.D
			part++
		}
	}

	return segments, nil
}

var dashTemplateTokenPattern = regexp.MustCompile(`\$(Time|Number)(?:%0?([0-9]+)d)?\$`)

func resolveDashTemplate(template string, part int64, segmentTime int64) (string, error) {
	found := false
	result := dashTemplateTokenPattern.ReplaceAllStringFunc(template, func(token string) string {
		found = true

		matches := dashTemplateTokenPattern.FindStringSubmatch(token)
		if len(matches) < 2 {
			return token
		}

		value := segmentTime
		if matches[1] == "Number" {
			value = part
		}

		if len(matches) >= 3 && matches[2] != "" {
			width, err := strconv.Atoi(matches[2])
			if err == nil && width > 0 {
				return fmt.Sprintf("%0*d", width, value)
			}
		}

		return strconv.FormatInt(value, 10)
	})
	if !found {
		return "", fmt.Errorf("dash media template must contain $Time$ or $Number$")
	}

	return result, nil
}

func safeArtifactPath(outputDir string, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("dash artifact name is required")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("dash artifact path must be relative: %q", name)
	}

	cleanName := filepath.Clean(name)
	if cleanName == "." || cleanName == string(filepath.Separator) || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) || cleanName == ".." {
		return "", fmt.Errorf("dash artifact path escapes output directory: %q", name)
	}

	return filepath.Join(outputDir, cleanName), nil
}

func nonEmptyFile(path string) (os.FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("path is a directory")
	}
	if stat.Size() == 0 {
		return nil, fmt.Errorf("file is empty")
	}

	return stat, nil
}

// ABOUTME: Enforces the security and publication contract of Chronicle's release configuration.
// ABOUTME: Keeps tag handling, workflow permissions, and Homebrew Cask publishing fail closed.

package releasepolicy

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseWorkflowUsesLeastPrivilege(t *testing.T) {
	workflow := loadYAML(t, ".github/workflows/release.yml")
	globalPermissions := mapping(t, workflow, "permissions")

	if _, ok := globalPermissions["packages"]; ok {
		t.Error("release workflow must not request unused packages permission")
	}
	if got := scalar(t, globalPermissions, "contents"); got != "read" {
		t.Errorf("global contents permission = %q, want read", got)
	}

	jobs := mapping(t, workflow, "jobs")
	for jobName, value := range jobs {
		job, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("job %q has type %T, want mapping", jobName, value)
		}
		if _, ok := optionalMapping(job, "permissions")["packages"]; ok {
			t.Errorf("job %q must not request unused packages permission", jobName)
		}
		if jobName != "release" && optionalScalar(optionalMapping(job, "permissions"), "contents") == "write" {
			t.Errorf("job %q must not request contents write permission", jobName)
		}
	}

	releaseJob := mapping(t, jobs, "release")
	releasePermissions := mapping(t, releaseJob, "permissions")
	if got := scalar(t, releasePermissions, "contents"); got != "write" {
		t.Errorf("release job contents permission = %q, want write", got)
	}
}

func TestReleaseWorkflowPassesTagThroughEnvironment(t *testing.T) {
	workflow := loadYAML(t, ".github/workflows/release.yml")
	foundTagEnvironment := false

	for jobName, value := range mapping(t, workflow, "jobs") {
		job, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("job %q has type %T, want mapping", jobName, value)
		}
		for _, step := range sequence(t, job, "steps") {
			run := optionalScalar(step, "run")
			if strings.Contains(run, "${{ github.ref_name }}") {
				t.Errorf("job %q step %q interpolates github.ref_name directly into shell", jobName, optionalScalar(step, "name"))
			}
			env := optionalMapping(step, "env")
			if optionalScalar(env, "TAG_NAME") == "${{ github.ref_name }}" && strings.Contains(run, `"$TAG_NAME"`) {
				foundTagEnvironment = true
			}
		}
	}

	if !foundTagEnvironment {
		t.Error("release workflow must pass github.ref_name through TAG_NAME and quote it in shell")
	}
}

func TestReleaseWorkflowValidatesInputsBeforePublishing(t *testing.T) {
	workflow := loadYAML(t, ".github/workflows/release.yml")
	releaseJob := mapping(t, mapping(t, workflow, "jobs"), "release")
	steps := sequence(t, releaseJob, "steps")
	validationIndex := stepIndex(steps, "Validate release inputs")
	releaseIndex := stepIndex(steps, "Create GitHub Release")

	if validationIndex == -1 {
		t.Fatal("release workflow has no input validation step")
	}
	if releaseIndex == -1 {
		t.Fatal("release workflow has no GitHub release step")
	}
	if validationIndex >= releaseIndex {
		t.Error("release inputs must be validated before the first publication step")
	}

	validationEnv := optionalMapping(steps[validationIndex], "env")
	if got := optionalScalar(validationEnv, "TAG_NAME"); got != "${{ github.ref_name }}" {
		t.Errorf("validation TAG_NAME = %q, want github.ref_name environment expression", got)
	}
	if got := optionalScalar(validationEnv, "HOMEBREW_TAP_TOKEN"); got != "${{ secrets.HOMEBREW_TAP_TOKEN }}" {
		t.Errorf("validation HOMEBREW_TAP_TOKEN = %q, want tap secret environment expression", got)
	}

	validationScript := scalar(t, steps[validationIndex], "run")
	validTags := []string{
		"v0.0.0",
		"v1.2.3",
		"v1.2.3-0",
		"v1.2.3-alpha",
		"v1.2.3-alpha.1",
		"v1.2.3-0.3.7",
		"v1.2.3-x.7.z.92",
		"v1.2.3-alpha-beta",
		"v1.2.3+build.1",
		"v1.2.3-alpha+001",
		"v1.2.3-rc.1+build.123",
	}
	for _, tag := range validTags {
		t.Run("accepts_"+tag, func(t *testing.T) {
			output, err := executeValidation(validationScript, tag, "present")
			if err != nil {
				t.Errorf("valid tag %q was rejected: %v\n%s", tag, err, output)
			}
		})
	}

	invalidTags := []string{
		"1.2.3",
		"v1.2",
		"v01.2.3",
		"v1.02.3",
		"v1.2.03",
		"v1.2.3-",
		"v1.2.3-01",
		"v1.2.3-00",
		"v1.2.3-.alpha",
		"v1.2.3-alpha.",
		"v1.2.3-alpha..1",
		"v1.2.3+",
		"v1.2.3+build.",
		"v1.2.3+build..1",
		"v1.2.3+_build",
		"v1.2.3-alpha_1",
		"v1.2.3;echo-injected",
	}
	for _, tag := range invalidTags {
		t.Run("rejects_"+tag, func(t *testing.T) {
			if output, err := executeValidation(validationScript, tag, "present"); err == nil {
				t.Errorf("invalid tag %q was accepted\n%s", tag, output)
			}
		})
	}

	t.Run("rejects_missing_tap_token", func(t *testing.T) {
		if output, err := executeValidation(validationScript, "v1.2.3", ""); err == nil {
			t.Errorf("missing tap token was accepted\n%s", output)
		}
	})
	if strings.Contains(validationScript, "${{ github.ref_name }}") {
		t.Error("validation shell directly interpolates github.ref_name")
	}
}

func TestTapPublicationIsFailClosedAndCaskOnly(t *testing.T) {
	workflow := loadYAML(t, ".github/workflows/release.yml")
	releaseJob := mapping(t, mapping(t, workflow, "jobs"), "release")
	steps := sequence(t, releaseJob, "steps")

	var tapStep map[string]any
	for _, step := range steps {
		if optionalScalar(step, "name") == "Update Homebrew Tap" {
			tapStep = step
			break
		}
	}
	if tapStep == nil {
		t.Fatal("release workflow has no Homebrew tap publication step")
	}

	run := optionalScalar(tapStep, "run")
	for _, forbidden := range []string{"Formula/", "< Formula", "||", "Push failed", "No changes to commit"} {
		if strings.Contains(run, forbidden) {
			t.Errorf("tap publication contains fail-open or Formula behavior %q", forbidden)
		}
	}
	for _, required := range []string{`Casks/chronicle.rb`, `: "${HOMEBREW_TAP_TOKEN:?`, `git commit`, `git push`} {
		if !strings.Contains(run, required) {
			t.Errorf("tap publication is missing required fail-closed Cask behavior %q", required)
		}
	}
	if optionalScalar(tapStep, "if") != "" {
		t.Error("tap publication must not be conditionally skipped")
	}
}

func TestGoReleaserPublishesOnlyHomebrewCasks(t *testing.T) {
	config := loadYAML(t, ".goreleaser.yml")
	if _, ok := config["brews"]; ok {
		t.Error("GoReleaser must not publish Homebrew Formulae")
	}
	if _, ok := config["homebrew_casks"]; !ok {
		t.Error("GoReleaser must publish a Homebrew Cask")
	}
}

func loadYAML(t *testing.T, relativePath string) map[string]any {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate release policy test")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	data, err := os.ReadFile(filepath.Join(projectRoot, relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse %s: %v", relativePath, err)
	}
	return document
}

func mapping(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing mapping %q", key)
	}
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%q has type %T, want mapping", key, value)
	}
	return result
}

func optionalMapping(parent map[string]any, key string) map[string]any {
	value, ok := parent[key]
	if !ok {
		return nil
	}
	result, _ := value.(map[string]any)
	return result
}

func sequence(t *testing.T, parent map[string]any, key string) []map[string]any {
	t.Helper()
	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing sequence %q", key)
	}
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("%q has type %T, want sequence", key, value)
	}
	result := make([]map[string]any, 0, len(items))
	for index, item := range items {
		mapping, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("%q item %d has type %T, want mapping", key, index, item)
		}
		result = append(result, mapping)
	}
	return result
}

func scalar(t *testing.T, parent map[string]any, key string) string {
	t.Helper()
	result := optionalScalar(parent, key)
	if result == "" {
		t.Fatalf("missing scalar %q", key)
	}
	return result
}

func optionalScalar(parent map[string]any, key string) string {
	if parent == nil {
		return ""
	}
	value, ok := parent[key]
	if !ok {
		return ""
	}
	result, _ := value.(string)
	return result
}

func stepIndex(steps []map[string]any, name string) int {
	for index, step := range steps {
		if optionalScalar(step, "name") == name {
			return index
		}
	}
	return -1
}

func executeValidation(script, tag, token string) ([]byte, error) {
	command := exec.Command("bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
	command.Env = []string{"HOMEBREW_TAP_TOKEN=" + token, "TAG_NAME=" + tag}
	return command.CombinedOutput()
}

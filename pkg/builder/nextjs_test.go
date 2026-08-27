package builder

import (
	"slices"
	"testing"
)

func TestDockerBuildPlatforms(t *testing.T) {
	t.Setenv("DOCKER_BUILD_PLATFORM", " linux/amd64, linux/arm64  linux/arm/v7 ")

	want := []string{"linux/amd64", "linux/arm64", "linux/arm/v7"}
	if got := dockerBuildPlatforms(); !slices.Equal(got, want) {
		t.Fatalf("dockerBuildPlatforms() = %v, want %v", got, want)
	}
}

func TestDockerBuildArgs(t *testing.T) {
	t.Setenv("DOCKER_BUILD_PLATFORM", "linux/amd64,linux/arm64")

	got, err := dockerBuildArgs("/workspace", "/workspace/Dockerfile", "registry.example.com/fyve/app:sha-1234567")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"buildx", "build",
		"--platform", "linux/amd64,linux/arm64",
		"--file", "/workspace/Dockerfile",
		"--tag", "registry.example.com/fyve/app:sha-1234567",
		"--tag", "registry.example.com/fyve/app:latest",
		"--cache-from", "type=registry,ref=registry.example.com/fyve/app:buildcache",
		"--cache-to", "type=registry,ref=registry.example.com/fyve/app:buildcache,mode=max,image-manifest=true,oci-mediatypes=true",
		"--push",
		"/workspace",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("dockerBuildArgs() = %v, want %v", got, want)
	}
}

func TestDockerBuildArgsDoesNotDuplicateLatestTag(t *testing.T) {
	t.Setenv("DOCKER_BUILD_PLATFORM", "linux/amd64")

	got, err := dockerBuildArgs(".", "Dockerfile", "localhost:5000/fyve/app:latest")
	if err != nil {
		t.Fatal(err)
	}

	for i, arg := range got {
		if arg == "--tag" && i+1 < len(got) && got[i+1] != "localhost:5000/fyve/app:latest" {
			t.Fatalf("unexpected additional tag %q", got[i+1])
		}
	}
}

func TestDockerBuildArgsRejectsUntaggedImage(t *testing.T) {
	if _, err := dockerBuildArgs(".", "Dockerfile", "registry.example.com/fyve/app"); err == nil {
		t.Fatal("dockerBuildArgs() accepted an untagged image")
	}
}

func TestDockerBuildArgsRejectsReservedCacheTag(t *testing.T) {
	if _, err := dockerBuildArgs(".", "Dockerfile", "registry.example.com/fyve/app:buildcache"); err == nil {
		t.Fatal("dockerBuildArgs() accepted the reserved buildcache tag")
	}
}

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hashText(value string) string {
	return hashBytes([]byte(value))
}

func shortID(prefix, seed string) string {
	return fmt.Sprintf("%s_%s", prefix, hashText(seed)[:16])
}

func slugify(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, `"`, "")
	slug = nonSlug.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func parseSkillRef(ref string) (owner string, slug string, err error) {
	parts := strings.Split(ref, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fail("Expected skill reference as <owner>/<skill>, got %q", ref)
	}
	owner = slugify(parts[0])
	slug = slugify(parts[1])
	if owner == "" || slug == "" {
		return "", "", fail("Expected skill reference as <owner>/<skill>, got %q", ref)
	}
	return owner, slug, nil
}

func nextPatchVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "0.1.0"
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "0.1.0"
	}
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
}

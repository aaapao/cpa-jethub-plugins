package main

import "testing"

// Case-only OAuth aliases are ignored by CPA (EqualFold), therefore CodeArts
// publishes the canonical public spelling itself and maps it back before the
// upstream call.
func TestPublicAndUpstreamModelID(t *testing.T) {
	if got := publicModelID("glm-5.3-flash"); got != "GLM-5.3-Flash" {
		t.Errorf("publicModelID = %q", got)
	}
	if got := upstreamModelID("GLM-5.3-Flash"); got != "glm-5.3-flash" {
		t.Errorf("upstreamModelID = %q", got)
	}
	info := modelInfoFor("glm-5.3-flash", "glm-5.3-flash")
	if info.ID != "GLM-5.3-Flash" || info.Name != "glm-5.3-flash" {
		t.Errorf("model info = %+v", info)
	}
}

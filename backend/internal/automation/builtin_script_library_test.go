package automation

import "testing"

func TestDefaultScriptBundles(t *testing.T) {
	bundles, err := DefaultScriptBundles()
	if err != nil {
		t.Fatalf("DefaultScriptBundles returned error: %v", err)
	}
	if len(bundles) != 3 {
		t.Fatalf("expected three default script bundles, got %d", len(bundles))
	}

	expected := []struct {
		id        string
		name      string
		uri       string
		publicAPI string
	}{
		{
			id:        DualInstanceRuntimeScriptID,
			name:      "双实例启动与 Runtime 切换",
			uri:       "repo://backend/internal/automation/demo-library/dual-instance-runtime-switch",
			publicAPI: "",
		},
		{
			id:        NewsQueryTXTScriptID,
			name:      "查询新闻并写 TXT",
			uri:       "repo://backend/internal/automation/demo-library/news-query-txt",
			publicAPI: "",
		},
		{
			id:        WebImageGenerateScriptID,
			name:      "网页图片生成并下载",
			uri:       "repo://backend/internal/automation/demo-library/web-image-generate-download",
			publicAPI: "image/chatgpt-generate-download",
		},
	}

	for index, item := range expected {
		bundle := bundles[index]
		if bundle.Record.ID != item.id {
			t.Fatalf("unexpected bundle id at %d: want %q got %q", index, item.id, bundle.Record.ID)
		}
		if bundle.Record.Name != item.name {
			t.Fatalf("unexpected bundle name at %d: want %q got %q", index, item.name, bundle.Record.Name)
		}
		if bundle.Record.EntryFile != "index.cjs" {
			t.Fatalf("unexpected entry file for %q: %q", bundle.Record.ID, bundle.Record.EntryFile)
		}
		if bundle.Record.Source.Type != "builtin" || bundle.Record.Source.URI != item.uri {
			t.Fatalf("unexpected source for %q: %+v", bundle.Record.ID, bundle.Record.Source)
		}
		if bundle.Record.PublicAPI.Path != item.publicAPI {
			t.Fatalf("unexpected public api path for %q: %+v", bundle.Record.ID, bundle.Record.PublicAPI)
		}
		if item.publicAPI != "" && !bundle.Record.PublicAPI.Enabled {
			t.Fatalf("expected public api to be enabled for %q", bundle.Record.ID)
		}
		if bundle.Record.ID == WebImageGenerateScriptID {
			if len(bundle.Record.PublicAPI.Variables) != 1 || bundle.Record.PublicAPI.Variables[0].Name != "prompt" {
				t.Fatalf("expected web image script to expose prompt variable, got %+v", bundle.Record.PublicAPI.Variables)
			}
		}
		if len(bundle.Files) == 0 {
			t.Fatalf("expected bundled files for %q", bundle.Record.ID)
		}
	}
}

func TestImportBuiltinBundleFromSourcePreservesManifestID(t *testing.T) {
	bundle, err := ImportBuiltinBundleFromSource(ScriptSource{
		Type: "builtin",
		Path: NewsQueryTXTScriptID,
	})
	if err != nil {
		t.Fatalf("ImportBuiltinBundleFromSource returned error: %v", err)
	}
	if bundle.Record.ID != NewsQueryTXTScriptID {
		t.Fatalf("expected manifest id %q, got %q", NewsQueryTXTScriptID, bundle.Record.ID)
	}
	if bundle.Record.Source.ImportedAt == "" {
		t.Fatalf("expected importedAt to be populated")
	}
}

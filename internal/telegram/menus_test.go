package telegram

import (
	"context"
	"testing"
)

func TestMenuItemsListenInTheirScene(t *testing.T) {
	b, _ := setupE2EBot(t)

	for _, spec := range b.menuSpecs() {
		if spec.scene == "" {
			t.Errorf("menu %q has no scene", spec.id)
		}
		for _, build := range spec.items {
			item := build(b)
			matcher, ok := item.(SceneMatcher)
			if !ok {
				continue
			}
			if scene := matcher.Scene(); scene != "" && scene != spec.scene {
				t.Errorf("menu %q: %s listens in scene %q, want %q", spec.id, item.Name(), scene, spec.scene)
			}
		}
	}
}

func TestMenusRegisterOpenersAndItems(t *testing.T) {
	b, _ := setupE2EBot(t)

	textCommands := map[string]bool{}
	for _, cmd := range b.textCommands {
		textCommands[cmd.Name()] = true
	}

	for _, spec := range b.menuSpecs() {
		switch {
		case spec.name != "":
			if b.commands["/"+spec.name] == nil {
				t.Errorf("menu %q did not register the /%s command", spec.id, spec.name)
			}
		case len(spec.openText) > 0:
			if !textCommands["/menu_"+spec.id] {
				t.Errorf("menu %q did not register a text opener", spec.id)
			}
		}
		for _, build := range spec.items {
			if !textCommands[build(b).Name()] {
				t.Errorf("menu %q did not register item %s", spec.id, build(b).Name())
			}
		}
	}
}

func TestMenuOpenersMatchTheirButtons(t *testing.T) {
	b, _ := setupE2EBot(t)

	for _, spec := range b.menuSpecs() {
		if spec.name == "" && len(spec.openText) == 0 {
			continue
		}
		opener := &menuCommand{bot: b, spec: spec}
		for _, text := range spec.openText {
			if !opener.MatchText(text) {
				t.Errorf("menu %q opener does not match its own button %q", spec.id, text)
			}
		}
	}
}

func TestOpeningEveryMenuSetsItsScene(t *testing.T) {
	b, repo := setupE2EBot(t)

	for i, spec := range b.menuSpecs() {
		if spec.name == "" && len(spec.openText) == 0 {
			continue
		}
		userID := int64(9000 + i)
		chat, _ := repo.FindOrCreate("telegram", userID)

		u := &Update{Bot: b, ChatID: userID, UserID: userID}
		if err := b.openMenu(u, chat, spec); err != nil {
			t.Errorf("menu %q: open failed: %v", spec.id, err)
			continue
		}

		saved, _ := repo.FindOrCreate("telegram", userID)
		if saved.Scene != spec.scene {
			t.Errorf("menu %q: scene = %q, want %q", spec.id, saved.Scene, spec.scene)
		}
	}
}

func TestEveryMenuSceneHasTextHandling(t *testing.T) {
	b, _ := setupE2EBot(t)

	handled := map[string]bool{sceneSetup: true}
	for _, cmd := range b.textCommands {
		if matcher, ok := cmd.(SceneMatcher); ok && matcher.Scene() != "" {
			handled[matcher.Scene()] = true
		}
	}

	for _, spec := range b.menuSpecs() {
		if len(spec.items) == 0 {
			continue
		}
		if !handled[spec.scene] {
			t.Errorf("menu %q declares items but its scene %q has no text handling", spec.id, spec.scene)
		}
	}
}

func TestMenuCommandIsHiddenWithoutPublicName(t *testing.T) {
	b, _ := setupE2EBot(t)

	for _, spec := range b.menuSpecs() {
		cmd := &menuCommand{bot: b, spec: spec}
		wantHidden := spec.name == ""
		if cmd.Hidden() != wantHidden {
			t.Errorf("menu %q: Hidden() = %v, want %v", spec.id, cmd.Hidden(), wantHidden)
		}
		if wantHidden && cmd.Name() != "/menu_"+spec.id {
			t.Errorf("menu %q: name = %q", spec.id, cmd.Name())
		}
	}
}

func TestMenuCommandDelegatesToOpenMenu(t *testing.T) {
	b, repo := setupE2EBot(t)

	spec, ok := b.menuByID("calls")
	if !ok {
		t.Fatal("calls menu not found")
	}
	cmd := &menuCommand{bot: b, spec: spec}

	userID := int64(9100)
	u := &Update{Bot: b, ChatID: userID, UserID: userID}
	if err := cmd.Handler(context.Background(), u); err != nil {
		t.Fatalf("handler: %v", err)
	}

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != sceneSettingsCalls {
		t.Errorf("scene = %q, want %q", saved.Scene, sceneSettingsCalls)
	}
}

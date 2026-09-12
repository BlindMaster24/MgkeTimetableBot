package telegram

import (
	"context"
	"testing"
)

func TestSceneRoutesCoverInputScenes(t *testing.T) {
	b, _ := setupE2EBot(t)

	inScenes := []string{
		sceneSetGroup,
		sceneSetTeacher,
		sceneSubAddGroup,
		sceneSubAddTeacher,
		sceneSubRemove,
		sceneHistoryTeacher,
		sceneHistoryWeek,
		sceneSubTestPick,
		sceneSubTestMode,
		sceneSubTestMode + ":teacher:Иванов И.И.",
		sceneCallsEditInput,
		sceneCallsEditReason,
		sceneCallsEditConfirm,
		sceneAliasAdd,
		sceneCompareStepA,
		sceneCompareInput,
		sceneCompareInput + ":100",
		sceneGetGroup + ":day",
		sceneGetTeacher + ":week",
	}
	for _, scene := range inScenes {
		matched := 0
		for _, route := range b.scenes {
			if route.match(scene) {
				matched++
			}
		}
		if matched != 1 {
			t.Errorf("scene %q matched %d routes, want exactly 1", scene, matched)
		}
	}

	notInScenes := []string{
		"",
		sceneSetup,
		sceneSettings,
		sceneSettingsSchedules,
		sceneSettingsCalls,
		sceneSettingsAlias,
		sceneGetGroup,
		sceneGetTeacher,
		"unknown_scene",
	}
	for _, scene := range notInScenes {
		for _, route := range b.scenes {
			if route.match(scene) {
				t.Errorf("scene %q must not be an input scene", scene)
			}
		}
	}
}

func TestDispatchInputSceneReturnsMatchState(t *testing.T) {
	b, repo := setupE2EBot(t)

	u := &Update{ChatID: 1, UserID: 1, Text: "что-то"}
	u.Bot = b

	chat := &Chat{Scene: "unknown_scene"}
	if b.dispatchInputScene(context.Background(), u, chat) {
		t.Error("unknown scene must not be dispatched")
	}

	chat, _ = repo.FindOrCreate("telegram", 1)
	chat.Scene = sceneSetup
	if b.dispatchInputScene(context.Background(), u, chat) {
		t.Error("setup scene must be handled by text commands, not the scene registry")
	}
}

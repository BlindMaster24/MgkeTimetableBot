package telegram

import "testing"

func TestSubscriptionsIgnoreDuplicates(t *testing.T) {
	repo, err := New(t.TempDir() + "/subs.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	created, err := repo.AddSubscription(7, "group", "100")
	if err != nil || !created {
		t.Fatalf("first add: created=%v err=%v", created, err)
	}
	created, err = repo.AddSubscription(7, "group", "100")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("the duplicate subscription must not be stored twice")
	}

	list, err := repo.GetSubscriptions(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Value != "100" {
		t.Fatalf("subscriptions = %+v", list)
	}
}

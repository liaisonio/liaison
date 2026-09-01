package web

import "testing"

func TestBuildMySQLMetadata_IncludesTablesForEveryVisibleDatabase(t *testing.T) {
	nodes := buildMySQLMetadata(
		[]string{"information_schema", "orders", "users"},
		[]webDataTableRef{
			{Namespace: "orders", Name: "invoices", Kind: "BASE TABLE"},
			{Namespace: "users", Name: "active_users", Kind: "VIEW"},
			{Namespace: "users", Name: "profiles", Kind: "BASE TABLE"},
		},
	)

	if len(nodes) != 1 || len(nodes[0].Children) != 3 {
		t.Fatalf("unexpected database tree: %#v", nodes)
	}
	orders := nodes[0].Children[1]
	if len(orders.Children) != 1 || orders.Children[0].Title != "invoices" {
		t.Fatalf("orders tables not populated: %#v", orders.Children)
	}
	users := nodes[0].Children[2]
	if len(users.Children) != 2 {
		t.Fatalf("users tables not populated: %#v", users.Children)
	}
	if !users.Children[0].HasChildren || users.Children[0].Children != nil {
		t.Fatalf("table columns should be lazy-loaded: %#v", users.Children[0])
	}
	if users.Children[0].Value != "view" {
		t.Fatalf("view marker missing: %#v", users.Children[0])
	}
}

func TestBuildPostgresMetadata_GroupsAllTablesBySchema(t *testing.T) {
	nodes := buildPostgresMetadata([]webDataTableRef{
		{Namespace: "tenant", Name: "accounts", Kind: "BASE TABLE"},
		{Namespace: "public", Name: "health", Kind: "VIEW"},
		{Namespace: "public", Name: "jobs", Kind: "BASE TABLE"},
	})

	if len(nodes) != 1 || len(nodes[0].Children) != 2 {
		t.Fatalf("unexpected schema tree: %#v", nodes)
	}
	public := nodes[0].Children[0]
	if public.Title != "public" || len(public.Children) != 2 {
		t.Fatalf("public schema not populated: %#v", public)
	}
	if public.Children[0].Value != "view" || !public.Children[0].HasChildren {
		t.Fatalf("postgres view should expose lazy fields: %#v", public.Children[0])
	}
}

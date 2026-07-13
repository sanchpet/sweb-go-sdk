package sweb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestDBaaSList(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{"can_create":true,"total_count":"8","max_count":110,"upgrade_agree":null,` +
			`"instances":[{"id":380,"billing_id":"testuser_dbaas_1","price":398,"active":true,` +
			`"ts_will_be_deleted":null,"blockUi":false,"currentAction":null,` +
			`"instance_uuid":"c000000-1111","name":"testuser_dbaas_1","display_name":"","status":"ready",` +
			`"engine":"MySQL 5.7","instances":1,"sync_replicas":0,"read_replicas":0,"replicas":0,` +
			`"is_enabled":true,"ip":"11.222.44.55:66666",` +
			`"plan":{"id":"2","name":"DBAAS-1/1/10","cpu":"1","memory":"1","storage":"10"},` +
			`"endpoints":[{"ip":"11.222.44.55","port":66666,"type":"rw"}],` +
			`"users":[{"name":"test"}],` +
			`"databases":[{"name":"test","size":0,"display_name":"","users":[]}]}]}}`))
	})
	idx, err := c.DBaaS.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "index" {
		t.Errorf("method = %q, want index", gotMethod)
	}
	if !idx.CanCreate || idx.TotalCount != 8 || idx.MaxCount != 110 || idx.UpgradeAgree != nil {
		t.Errorf("meta = %+v, want can_create/8/110/nil", idx)
	}
	if len(idx.Instances) != 1 {
		t.Fatalf("instances = %d, want 1", len(idx.Instances))
	}
	inst := idx.Instances[0]
	if inst.ID != 380 || inst.BillingID != "testuser_dbaas_1" || inst.Price != 398 ||
		inst.Plan.Name != "DBAAS-1/1/10" || inst.Plan.CPU != 1 ||
		len(inst.Endpoints) != 1 || inst.Endpoints[0].Port != 66666 || inst.Endpoints[0].Type != "rw" ||
		len(inst.Databases) != 1 || inst.Databases[0].Name != "test" ||
		len(inst.Users) != 1 || inst.Users[0].Name != "test" {
		t.Errorf("instance = %+v", inst)
	}
}

func TestDBaaSSetUpgradeAgree(t *testing.T) {
	var gotMethod string
	var gotAgree bool
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				UpgradeAgree bool `json:"upgradeAgree"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotAgree = req.Method, req.Params.UpgradeAgree
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := c.DBaaS.SetUpgradeAgree(context.Background(), true); err != nil {
		t.Fatalf("SetUpgradeAgree: %v", err)
	}
	if gotMethod != "setUpgradeAgree" || !gotAgree {
		t.Errorf("method/agree = %q/%v, want setUpgradeAgree/true", gotMethod, gotAgree)
	}
}

func TestDBaaSSetUpgradeAgreeFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := c.DBaaS.SetUpgradeAgree(context.Background(), false); err == nil {
		t.Error("SetUpgradeAgree: want error on 0 sentinel, got nil")
	}
}

func TestDBaaSAvailableConfig(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{` +
			`"plans":[{"id":"1","name":"Тестовый","cpu":"2","memory":"2","storage":"40"}],` +
			`"engines":{"MySQL":[{"name":"MySQL 5.7","version":"5.7"}],` +
			`"PostgreSQL":[{"name":"PostgreSQL 13","version":"13"},{"name":"PostgreSQL 14","version":"14"}]},` +
			`"kit":{"categoryId":"dbaas","cpu":{"start":1,"end":8,"step":1,"price":null}}}}`))
	})
	cfg, err := c.DBaaS.AvailableConfig(context.Background())
	if err != nil {
		t.Fatalf("AvailableConfig: %v", err)
	}
	if gotMethod != "getAvailableConfig" {
		t.Errorf("method = %q, want getAvailableConfig", gotMethod)
	}
	if len(cfg.Plans) != 1 || cfg.Plans[0].Name != "Тестовый" || cfg.Plans[0].CPU != 2 {
		t.Errorf("plans = %+v", cfg.Plans)
	}
	if len(cfg.Engines["PostgreSQL"]) != 2 || cfg.Engines["PostgreSQL"][0].Version != "13" ||
		len(cfg.Engines["MySQL"]) != 1 {
		t.Errorf("engines = %+v", cfg.Engines)
	}
	if len(cfg.Kit) == 0 {
		t.Error("kit = empty, want raw JSON preserved")
	}
}

func TestDBaaSConstructorPlanID(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		CPU      int `json:"cpu"`
		Memory   int `json:"memory"`
		Storage  int `json:"storage"`
		Replicas int `json:"replicas"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":96}`))
	})
	id, err := c.DBaaS.ConstructorPlanID(context.Background(), 1, 2, 160, 1)
	if err != nil {
		t.Fatalf("ConstructorPlanID: %v", err)
	}
	if gotMethod != "getConstructorPlanId" || id != 96 {
		t.Errorf("method/id = %q/%d, want getConstructorPlanId/96", gotMethod, id)
	}
	if gotParams.CPU != 1 || gotParams.Memory != 2 || gotParams.Storage != 160 || gotParams.Replicas != 1 {
		t.Errorf("params = %+v, want 1/2/160/1", gotParams)
	}
}

func TestDBaaSGetFirstOrderInfo(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		// Mixed quoting mirrors the recorded example: cpu/memory/storage/*_replicas
		// quoted, replicas/instances/pay_period bare, promocode null.
		_, _ = w.Write([]byte(`{"result":{"clearAvailable":true,"cpu":"1","engine":"PostgreSQL 13",` +
			`"engine_type":"PostgreSQL","engine_version":"13","instances":1,"memory":"1","pay_period":1,` +
			`"plan":"DBAAS-1/1/10","plan_is_constructor":true,"price_per_month":398,"promocode":null,` +
			`"read_replicas":"0","replicas":0,"storage":"10","sync_replicas":"0"}}`))
	})
	info, err := c.DBaaS.GetFirstOrderInfo(context.Background())
	if err != nil {
		t.Fatalf("GetFirstOrderInfo: %v", err)
	}
	if gotMethod != "getFirstOrderInfo" {
		t.Errorf("method = %q, want getFirstOrderInfo", gotMethod)
	}
	if info.Plan != "DBAAS-1/1/10" || info.EngineType != "PostgreSQL" || info.CPU != 1 ||
		info.Memory != 1 || info.Storage != 10 || info.PricePerMonth != 398 ||
		info.PayPeriod != 1 || !info.PlanIsConstructor || !info.ClearAvailable || info.Promocode != "" {
		t.Errorf("info = %+v", info)
	}
}

func TestDBaaSRemoveFirst(t *testing.T) {
	var gotMethod string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	raw, err := c.DBaaS.RemoveFirst(context.Background())
	if err != nil {
		t.Fatalf("RemoveFirst: %v", err)
	}
	if gotMethod != "removeFirst" || string(raw) != "1" {
		t.Errorf("method/raw = %q/%s, want removeFirst/1", gotMethod, raw)
	}
}

func TestDBaaSCreateInstance(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		EngineType    string                 `json:"engineType"`
		EngineVersion string                 `json:"engineVersion"`
		Users         []DBaaSUserCredentials `json:"users"`
		PlanID        int                    `json:"planId"`
		DisplayName   string                 `json:"displayName"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":{"extendedResult":{"code":1,"data":[],"message":"Создание кластера баз данных запущено"}}}`))
	})
	raw, err := c.DBaaS.CreateInstance(context.Background(), CreateInstanceRequest{
		EngineType:    "PostgreSQL",
		EngineVersion: "13",
		Users:         []DBaaSUserCredentials{{Name: "test1", Password: "Pass_ByTest1"}},
		PlanID:        3,
		DisplayName:   "Тестовая",
	})
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if gotMethod != "createInstance" || gotParams.EngineType != "PostgreSQL" ||
		gotParams.EngineVersion != "13" || gotParams.PlanID != 3 || gotParams.DisplayName != "Тестовая" ||
		len(gotParams.Users) != 1 || gotParams.Users[0].Name != "test1" || gotParams.Users[0].Password != "Pass_ByTest1" {
		t.Errorf("method/params = %q/%+v", gotMethod, gotParams)
	}
	if len(raw) == 0 {
		t.Error("raw result = empty, want extendedResult preserved")
	}
}

func TestDBaaSEditInstance(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID   string `json:"billingId"`
		PlanID      int    `json:"planId"`
		DisplayName string `json:"displayName"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	if err := c.DBaaS.EditInstance(context.Background(), EditInstanceRequest{
		BillingID:   "testuser_dbaas_1",
		PlanID:      3,
		DisplayName: "Изменённая",
	}); err != nil {
		t.Fatalf("EditInstance: %v", err)
	}
	if gotMethod != "editInstance" || gotParams.BillingID != "testuser_dbaas_1" ||
		gotParams.PlanID != 3 || gotParams.DisplayName != "Изменённая" {
		t.Errorf("method/params = %q/%+v", gotMethod, gotParams)
	}
}

func TestDBaaSEditInstanceFailure(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	})
	if err := c.DBaaS.EditInstance(context.Background(), EditInstanceRequest{BillingID: "x"}); err == nil {
		t.Error("EditInstance: want error on 0 sentinel, got nil")
	}
}

func TestDBaaSRemoveInstance(t *testing.T) {
	var gotMethod, gotBillingID string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BillingID string `json:"billingId"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotBillingID = req.Method, req.Params.BillingID
		_, _ = w.Write([]byte(`{"result":1}`))
	})
	raw, err := c.DBaaS.RemoveInstance(context.Background(), "testuser_dbaas_1")
	if err != nil {
		t.Fatalf("RemoveInstance: %v", err)
	}
	if gotMethod != "removeInstance" || gotBillingID != "testuser_dbaas_1" || string(raw) != "1" {
		t.Errorf("method/billingId/raw = %q/%q/%s", gotMethod, gotBillingID, raw)
	}
}

func TestDBaaSDeleteDatabase(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		DBName    string `json:"dbName"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":{"extendedResult":{"code":1,"data":[],"message":"База данных удалена"}}}`))
	})
	raw, err := c.DBaaS.DeleteDatabase(context.Background(), "testuser_dbaas_1", "test1")
	if err != nil {
		t.Fatalf("DeleteDatabase: %v", err)
	}
	if gotMethod != "deleteDatabase" || gotParams.BillingID != "testuser_dbaas_1" || gotParams.DBName != "test1" {
		t.Errorf("method/params = %q/%+v", gotMethod, gotParams)
	}
	if len(raw) == 0 {
		t.Error("raw result = empty, want extendedResult preserved")
	}
}

func TestDBaaSValidateUsers(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		Users []DBaaSUserCredentials `json:"users"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"result":true}`))
	})
	if err := c.DBaaS.ValidateUsers(context.Background(), []DBaaSUserCredentials{{Name: "test1", Password: "Pass_ByTest1"}}); err != nil {
		t.Fatalf("ValidateUsers: %v", err)
	}
	if gotMethod != "validateUsers" || len(gotParams.Users) != 1 || gotParams.Users[0].Name != "test1" {
		t.Errorf("method/params = %q/%+v", gotMethod, gotParams)
	}
}

package ip

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sanchpet/sweb-go-sdk/internal/transport"
)

func TestAddLocal(t *testing.T) {
	var gotMethod, gotBilling string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BillingID string `json:"billingId"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod, gotBilling = req.Method, req.Params.BillingID
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := s.AddLocal(context.Background(), "login_vps_1"); err != nil {
		t.Fatalf("AddLocal: %v", err)
	}
	if gotMethod != "addLocal" || gotBilling != "login_vps_1" {
		t.Errorf("method/billing = %q/%q, want addLocal/login_vps_1", gotMethod, gotBilling)
	}
}

func TestRemoveLocal(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := s.RemoveLocal(context.Background(), "login_vps_1"); err != nil {
		t.Fatalf("RemoveLocal: %v", err)
	}
	if gotMethod != "removeLocal" {
		t.Errorf("method = %q, want removeLocal", gotMethod)
	}
}

func TestAddLocalFailure(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0}`))
	})
	if err := s.AddLocal(context.Background(), "login_vps_1"); err == nil {
		t.Fatal("AddLocal: want error on result 0, got nil")
	}
}

func TestIPInfoLocalIP(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		// Real attached shape: local_ip and ips come as BARE OBJECTS (not arrays) when
		// populated; a public IP has a FRACTIONAL price (142.06, money-as-float).
		_, _ = w.Write([]byte(`{"result":{"ips":{"ip":"77.222.43.225","gateway":"77.222.43.1","netmask":"77.222.43.0/24","datacenter":1,"ptr":"x","price":142.06},"protected_ips":[],"local_ip":{"ip":"10.0.0.24","mac":"00:16:3e:aa:bb:cc","mask":"10.0.0.0/27"},"vps":{"billingId":"login_vps_1","currentAction":null,"isEmpty":"0","ordered_ip_count":"2"}}}`))
	})
	info, err := s.Info(context.Background(), "login_vps_1")
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if len(info.LocalIP) != 1 || info.LocalIP[0].IP != "10.0.0.24" || info.LocalIP[0].Mask != "10.0.0.0/27" {
		t.Errorf("local_ip = %+v, want one 10.0.0.24 /27", info.LocalIP)
	}
	if len(info.IPs) != 1 || info.IPs[0].Price < 142 || info.IPs[0].Price > 143 {
		t.Errorf("ips[0].price = %v, want ~142.06 (fractional, FlexFloat)", info.IPs)
	}
	if info.VPS.OrderedIPCount != 2 {
		t.Errorf("ordered_ip_count = %d, want 2", int64(info.VPS.OrderedIPCount))
	}
}

func TestAddIP(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		Number    int    `json:"number"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"ip":"203.0.113.7"}}`))
	})
	if _, err := s.Add(context.Background(), "login_vps_1", 2); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if gotMethod != "add" || gotParams.BillingID != "login_vps_1" || gotParams.Number != 2 {
		t.Errorf("method/params = %q/%+v, want add / login_vps_1,2", gotMethod, gotParams)
	}
}

func TestRemoveIP(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		IP        string `json:"ip"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := s.Remove(context.Background(), "login_vps_1", "203.0.113.7"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if gotMethod != "remove" || gotParams.IP != "203.0.113.7" {
		t.Errorf("method/ip = %q/%q, want remove/203.0.113.7", gotMethod, gotParams.IP)
	}
}

func TestMoveIP(t *testing.T) {
	// Attach sends billingId; detach (empty) sends null.
	for _, tc := range []struct {
		name      string
		billingID string
		wantNull  bool
	}{
		{"attach", "login_vps_2", false},
		{"detach", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rawParams json.RawMessage
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
					Params json.RawMessage
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				rawParams = req.Params
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
			})
			if err := s.Move(context.Background(), "203.0.113.7", tc.billingID); err != nil {
				t.Fatalf("Move: %v", err)
			}
			var p struct {
				BillingID *string `json:"billingId"`
			}
			_ = json.Unmarshal(rawParams, &p)
			if tc.wantNull && p.BillingID != nil {
				t.Errorf("detach: billingId = %v, want null", *p.BillingID)
			}
			if !tc.wantNull && (p.BillingID == nil || *p.BillingID != tc.billingID) {
				t.Errorf("attach: billingId = %v, want %q", p.BillingID, tc.billingID)
			}
		})
	}
}

func TestEditPtr(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		IP  string `json:"ip"`
		PTR string `json:"ptr"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := s.EditPtr(context.Background(), "203.0.113.7", "host.example.com"); err != nil {
		t.Fatalf("EditPtr: %v", err)
	}
	if gotMethod != "editPtr" || gotParams.IP != "203.0.113.7" || gotParams.PTR != "host.example.com" {
		t.Errorf("method/params = %q/%+v, want editPtr / ip,ptr", gotMethod, gotParams)
	}
}

func TestGetPtr(t *testing.T) {
	// Tolerate both a bare-string result and a {"ptr": …} object.
	for _, tc := range []struct{ name, result string }{
		{"string", `"host.example.com"`},
		{"object", `{"ptr":"host.example.com"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + tc.result + `}`))
			})
			ptr, err := s.GetPtr(context.Background(), "203.0.113.7")
			if err != nil {
				t.Fatalf("GetPtr: %v", err)
			}
			if ptr != "host.example.com" {
				t.Errorf("ptr = %q, want host.example.com", ptr)
			}
		})
	}
}

func TestGetAllIPList(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		// Two rows: an ordinary attached IP and a free protected IP whose
		// nullable fields (name, billingId) really arrive as null; datacenter
		// and price come as bare numbers, planId/limit as int or null.
		_, _ = w.Write([]byte(`{"result":[` +
			`{"ip":"203.0.113.7","name":"acct_vps_1","billingId":"acct_vps_1","datacenter":1,"gateway":"203.0.113.1","netmask":"203.0.113.0/24","isPrimary":false,"allowBeDecline":true,"canBeDecline":true,"canBeMove":true,"currentAction":null,"acceptorBillingIds":[],"price":140,"date":"11.06.2025","planId":null,"limit":null},` +
			`{"ip":"203.0.113.44","name":null,"billingId":null,"datacenter":1,"gateway":"203.0.113.1","canBeMove":true,"currentAction":null,"acceptorBillingIds":[{"billingId":"acct_vps_1","name":"acct_vps_1"}],"netmask":"203.0.113.0/24","isPrimary":false,"allowBeDecline":true,"canBeDecline":true,"price":6000,"date":null,"planId":2,"limit":50}` +
			`]}`))
	})
	list, err := s.GetAllIPList(context.Background())
	if err != nil {
		t.Fatalf("GetAllIPList: %v", err)
	}
	if gotMethod != "getAllIpList" {
		t.Errorf("method = %q, want getAllIpList", gotMethod)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].IP != "203.0.113.7" || list[0].BillingID != "acct_vps_1" || list[0].Price != 140 {
		t.Errorf("row0 = %+v, want 203.0.113.7 / acct_vps_1 / 140", list[0])
	}
	p := list[1]
	if p.Name != "" || p.BillingID != "" {
		t.Errorf("row1 nullable name/billingId = %q/%q, want empty", p.Name, p.BillingID)
	}
	if p.PlanID != 2 || p.Limit != 50 {
		t.Errorf("row1 planId/limit = %d/%d, want 2/50", int64(p.PlanID), int64(p.Limit))
	}
	if len(p.AcceptorBillingIDs) != 1 || p.AcceptorBillingIDs[0].BillingID != "acct_vps_1" {
		t.Errorf("row1 acceptorBillingIds = %+v, want one acct_vps_1", p.AcceptorBillingIDs)
	}
}

func TestGetOrderInfo(t *testing.T) {
	var gotMethod string
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_, _ = w.Write([]byte(`{"result":{"dailyIpLimit":7,"dailyProtectedIpLimit":7,"ipOrdersLastDay":0,"protectedIpOrdersLastDay":3}}`))
	})
	info, err := s.GetOrderInfo(context.Background())
	if err != nil {
		t.Fatalf("GetOrderInfo: %v", err)
	}
	if gotMethod != "getOrderInfo" {
		t.Errorf("method = %q, want getOrderInfo", gotMethod)
	}
	if info.DailyIPLimit != 7 || info.ProtectedIPOrdersLastDay != 3 || info.IPOrdersLastDay != 0 {
		t.Errorf("info = %+v, want dailyIpLimit 7 / protectedOrders 3 / ipOrders 0", info)
	}
}

func TestAddProtected(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		BillingID string `json:"billingId"`
		PlanIDs   []int  `json:"planIds"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":true}`))
	})
	if err := s.AddProtected(context.Background(), "login_vps_1", []int{2, 3}); err != nil {
		t.Fatalf("AddProtected: %v", err)
	}
	if gotMethod != "addProtected" || gotParams.BillingID != "login_vps_1" || len(gotParams.PlanIDs) != 2 || gotParams.PlanIDs[0] != 2 {
		t.Errorf("method/params = %q/%+v, want addProtected / login_vps_1,[2 3]", gotMethod, gotParams)
	}
}

func TestRemoveProtected(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		IP string `json:"ip"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":true}`))
	})
	if err := s.RemoveProtected(context.Background(), "127.0.105.44"); err != nil {
		t.Fatalf("RemoveProtected: %v", err)
	}
	if gotMethod != "removeProtected" || gotParams.IP != "127.0.105.44" {
		t.Errorf("method/ip = %q/%q, want removeProtected/127.0.105.44", gotMethod, gotParams.IP)
	}
}

// The protected sentinel is documented as boolean but removeProtected's result
// $ref points at the integer resultAdd; accept integer 1 as success too.
func TestProtectedIntegerSentinel(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":1}`))
	})
	if err := s.RemoveProtected(context.Background(), "127.0.105.44"); err != nil {
		t.Fatalf("RemoveProtected with integer 1: %v", err)
	}
}

func TestProtectedFailure(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":false}`))
	})
	if err := s.UpdateProtected(context.Background(), "127.0.105.44", 3); err == nil {
		t.Fatal("UpdateProtected: want error on result false, got nil")
	}
}

func TestUpdateProtected(t *testing.T) {
	var gotMethod string
	var gotParams struct {
		IP     string `json:"ip"`
		PlanID int    `json:"planId"`
	}
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params json.RawMessage
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotMethod = req.Method
		_ = json.Unmarshal(req.Params, &gotParams)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":true}`))
	})
	if err := s.UpdateProtected(context.Background(), "127.0.105.44", 3); err != nil {
		t.Fatalf("UpdateProtected: %v", err)
	}
	if gotMethod != "updateProtected" || gotParams.IP != "127.0.105.44" || gotParams.PlanID != 3 {
		t.Errorf("method/params = %q/%+v, want updateProtected / ip,planId 3", gotMethod, gotParams)
	}
}

func TestMoveProtected(t *testing.T) {
	// Attach sends billingId; detach (empty) sends null.
	for _, tc := range []struct {
		name      string
		billingID string
		wantNull  bool
	}{
		{"attach", "login_vps_2", false},
		{"detach", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			var rawParams json.RawMessage
			s := serve(t, func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Method string `json:"method"`
					Params json.RawMessage
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotMethod = req.Method
				rawParams = req.Params
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":true}`))
			})
			if err := s.MoveProtected(context.Background(), "127.0.105.44", tc.billingID); err != nil {
				t.Fatalf("MoveProtected: %v", err)
			}
			if gotMethod != "moveProtected" {
				t.Errorf("method = %q, want moveProtected", gotMethod)
			}
			var p struct {
				BillingID *string `json:"billingId"`
			}
			_ = json.Unmarshal(rawParams, &p)
			if tc.wantNull && p.BillingID != nil {
				t.Errorf("detach: billingId = %v, want null", *p.BillingID)
			}
			if !tc.wantNull && (p.BillingID == nil || *p.BillingID != tc.billingID) {
				t.Errorf("attach: billingId = %v, want %q", p.BillingID, tc.billingID)
			}
		})
	}
}

func TestWaitForLocalIP(t *testing.T) {
	var calls int
	s := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 2 {
			_, _ = w.Write([]byte(`{"result":{"local_ip":[],"vps":{"billingId":"login_vps_1"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"local_ip":[{"ip":"10.0.0.24","mac":"x","mask":"10.0.0.0/27"}],"vps":{"billingId":"login_vps_1"}}}`))
	})
	lip, err := s.WaitForLocalIP(context.Background(), "login_vps_1", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForLocalIP: %v", err)
	}
	if lip.IP != "10.0.0.24" {
		t.Errorf("local ip = %q, want 10.0.0.24", lip.IP)
	}
	if calls < 2 {
		t.Errorf("calls = %d, want >= 2 (polled)", calls)
	}
}

// serve spins up a mock JSON-RPC server for h and returns an ip.Service backed
// by a transport pointed at it.
func serve(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(transport.New(
		transport.WithBaseURL(srv.URL),
		transport.WithHTTPClient(srv.Client()),
		transport.WithToken("test-token"),
	))
}

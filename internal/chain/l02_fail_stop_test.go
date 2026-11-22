package chain
import("errors";"testing";"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage")
func TestAcceptOrdinaryRejectionsDoNotWrapSentinel(t *testing.T){
 st,err:=storage.Open(t.TempDir());if err!=nil{t.Fatal(err)};defer st.Close();ch,err:=Open(st);if err!=nil{t.Fatal(err)}
 ok,reason,err:=ch.Accept(ch.Tip());if err!=nil||ok||reason!="duplicate"{t.Fatalf("duplicate Accept = (%v, %q, %v), want false, duplicate, nil",ok,reason,err)}
 if errors.Is(err,storage.ErrStoreFailed){t.Fatalf("ordinary duplicate rejection wraps storage sentinel: %v",err)}
}

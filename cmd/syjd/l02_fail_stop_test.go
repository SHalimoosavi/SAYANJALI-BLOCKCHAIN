package main
import("context";"errors";"net/http";"testing";"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/node";"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage")
type l02FakeStartNode struct{done chan struct{};cause error;stopped bool}
func(f *l02FakeStartNode)Start(context.Context)error{return nil}
func(f *l02FakeStartNode)Done()<-chan struct{}{return f.done}
func(f *l02FakeStartNode)Err()error{return f.cause}
func(f *l02FakeStartNode)Stop(){f.stopped=true}
func(f *l02FakeStartNode)ServeAPI()*http.Server{return &http.Server{}}
func(f *l02FakeStartNode)Status()map[string]any{return map[string]any{}}
func TestRunStartReturnsErrorOnFatalStorageFailure(t *testing.T){
 cause:=errors.Join(errors.New("fatal storage failure"),storage.ErrStoreFailed);f:=&l02FakeStartNode{done:make(chan struct{}),cause:cause};close(f.done)
 err:=runStartWith(context.Background(),node.DefaultConfig(t.TempDir()),f,nil)
 if !errors.Is(err,storage.ErrStoreFailed){t.Fatalf("want error wrapping storage.ErrStoreFailed, got %v",err)}
}
func TestNormalSignalShutdownExitsZero(t *testing.T){
 ctx,cancel:=context.WithCancel(context.Background());cancel();f:=&l02FakeStartNode{done:make(chan struct{})}
 if err:=runStartWith(ctx,node.DefaultConfig(t.TempDir()),f,nil);err!=nil{t.Fatalf("normal signal shutdown returned error: %v",err)}
 if !f.stopped{t.Fatal("normal shutdown did not stop node")}
}

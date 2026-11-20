package app

import (
    "strings"
    "testing"
)

// The stage-transition guard must reject an unreferenced payout before
// opening a database transaction or changing the initiative stage.
func TestP0PaymentReferenceRequired(t *testing.T) {
    a:=newTestApp(t,false)
    i:=&Initiative{ID:999999,StageCode:StageImplement,RecordStatus:StatusOpen}
    inputs:=[]ReviewDecisionInput{
      {Decision:"APPROVE",ImplementationStatus:"IMPLEMENTED",PaymentReference:""},
      {Decision:"APPROVE",ImplementationStatus:"IMPLEMENTED",PaymentReference:"123"},
      {Decision:"APPROVE",ImplementationStatus:"IMPLEMENTED",PaymentReference:"PAY\nX2026"},
      {Decision:"APPROVE",ImplementationStatus:"IMPLEMENTED",PaymentReference:strings.Repeat("A",121)},
    }
    for _,input:=range inputs {
      err:=a.applyStageReview(i,"CF-0001",input)
      if err==nil||!strings.Contains(err.Error(),"chứng từ") {
        t.Fatalf("missing/invalid payment evidence unexpectedly accepted: %+v err=%v",input,err)
      }
    }
}

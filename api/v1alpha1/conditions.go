package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func ConditionFromReason(reason ReasonWithMessage) *metav1.Condition {
	condition, found := conditionReasons[reason.Reason]
	if found {
		message := condition.Message
		if reason.Message != "" {
			message = reason.Message
		}
		return &metav1.Condition{
			Type:    string(condition.Type),
			Status:  condition.Status,
			Reason:  string(reason.Reason),
			Message: message,
		}
	}
	return nil
}

var conditionReasons = map[ConditionReason]conditionMeta{
	ConditionReasonUpdateSucceeded: {Type: ConditionTypeReady, Status: metav1.ConditionTrue, Message: ConditionReasonUpdateSucceededMessage},
	ConditionReasonUpdateFailed:    {Type: ConditionTypeReady, Status: metav1.ConditionFalse, Message: ConditionReasonUpdateFailedMessage},
}

type conditionMeta struct {
	Type    ConditionType
	Status  metav1.ConditionStatus
	Message string
}

func NewReasonWithMessage(reason ConditionReason, customMessage ...string) ReasonWithMessage {
	message := ""
	if len(customMessage) > 0 {
		message = customMessage[0]
	}
	return ReasonWithMessage{
		Reason:  reason,
		Message: message,
	}
}

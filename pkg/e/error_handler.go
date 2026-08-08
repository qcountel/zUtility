package e

type ErrorHandler interface {
	HandleError(source string, err error)
}

var _ ErrorHandler = (*NopErrorHandler)(nil)

type NopErrorHandler struct{}

func (NopErrorHandler) HandleError(string, error) {}

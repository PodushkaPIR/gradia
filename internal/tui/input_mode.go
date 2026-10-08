package tui

type inputMode uint8

const (
	inputNavigation inputMode = iota
	inputText
	inputConfirmation
	inputBusy
)

func (m inputMode) allowsHelp() bool {
	return m == inputNavigation || m == inputConfirmation
}

func (m inputMode) allowsQuit() bool {
	return m == inputNavigation
}

func (m inputMode) allowsScreenSwitch() bool {
	return m == inputNavigation
}

func (m inputMode) allowsCancel() bool {
	return m == inputText || m == inputConfirmation
}

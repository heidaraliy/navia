package ui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	TopBar          lipgloss.Style
	Logo            lipgloss.Style
	Brand           lipgloss.Style
	Footer          lipgloss.Style
	Pane            lipgloss.Style
	TreePane        lipgloss.Style
	Panel           lipgloss.Style
	Focused         lipgloss.Style
	Blurred         lipgloss.Style
	Selected        lipgloss.Style
	SelectedBlurred lipgloss.Style
	TreeDim         lipgloss.Style
	DirDim          lipgloss.Style
	Dim             lipgloss.Style
	Dir             lipgloss.Style
	Error           lipgloss.Style
	Modal           lipgloss.Style
	Command         lipgloss.Style
	Highlight       lipgloss.Style
}

func NewStyles() Styles {
	return Styles{
		TopBar:          lipgloss.NewStyle().Foreground(lipgloss.Color("230")),
		Logo:            lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
		Brand:           lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
		Footer:          lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("238")).Padding(0, 1),
		Pane:            lipgloss.NewStyle().Padding(0, 1),
		TreePane:        lipgloss.NewStyle(),
		Panel:           lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39")).Padding(0, 1),
		Focused:         lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("39")).Padding(0, 1),
		Blurred:         lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("24")).Padding(0, 1),
		Selected:        lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")),
		SelectedBlurred: lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("235")),
		TreeDim:         lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		DirDim:          lipgloss.NewStyle().Foreground(lipgloss.Color("67")),
		Dim:             lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Dir:             lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true),
		Error:           lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		Modal:           lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Background(lipgloss.Color("235")),
		Command:         lipgloss.NewStyle().Foreground(lipgloss.Color("151")),
		Highlight:       lipgloss.NewStyle().Foreground(lipgloss.Color("229")),
	}
}

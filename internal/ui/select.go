package ui

import "github.com/charmbracelet/huh"

// SelectOption presents an interactive arrow-key menu
func SelectOption(prompt string, options []string) (string, error) {
	var selected string

	huhOptions := make([]huh.Option[string], len(options))
	for i, opt := range options {
		huhOptions[i] = huh.NewOption(opt, opt)
	}

	err := huh.NewSelect[string]().
		Title(prompt).
		Options(huhOptions...).
		Value(&selected).
		Run()

	return selected, err
}

// PromptInput presents an interactive text input
func PromptInput(prompt, placeholder string) (string, error) {
	var input string

	err := huh.NewInput().
		Title(prompt).
		Placeholder(placeholder).
		Value(&input).
		Run()

	return input, err
}

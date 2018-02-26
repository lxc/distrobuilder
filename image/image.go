package image

import pongo2 "gopkg.in/flosch/pongo2.v3"

func renderTemplate(template string, ctx pongo2.Context) (string, error) {
	var (
		err error
		ret string
	)

	// Load template from string
	tpl, err := pongo2.FromString(template)
	if err != nil {
		return ret, err
	}

	// Get rendered template
	ret, err = tpl.Execute(ctx)
	if err != nil {
		return ret, err
	}

	return ret, err
}

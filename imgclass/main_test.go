package main

import (
	"context"
	"testing"

	"github.com/matryer/is"
)

func TestClasses(t *testing.T) {
	is := is.New(t)

	classes, err := collectTrainingData(context.Background(), "testdata/catsdogs")
	is.NoErr(err)
	is.Equal(len(classes), 2)
	is.Equal(len(classes["cats"]), 3)
	is.Equal(classes["cats"][0], "testdata/catsdogs/cats/cat1.jpg")
	is.Equal(classes["cats"][1], "testdata/catsdogs/cats/cat2.jpg")
	is.Equal(classes["cats"][2], "testdata/catsdogs/cats/cat3.jpg")
	is.Equal(len(classes["dogs"]), 3)
	is.Equal(classes["dogs"][0], "testdata/catsdogs/dogs/dog1.jpg")
	is.Equal(classes["dogs"][1], "testdata/catsdogs/dogs/dog2.jpg")
	is.Equal(classes["dogs"][2], "testdata/catsdogs/dogs/dog3.jpg")
}

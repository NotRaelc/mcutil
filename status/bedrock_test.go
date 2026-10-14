package status_test

import (
	"context"
	"testing"

	"github.com/NotRaelc/mcutil/v4/status"
	"github.com/NotRaelc/mcutil/v4/util"
)

func TestBedrock(t *testing.T) {
	resp, err := status.Bedrock(context.Background(), "lifesteal.net", util.DefaultBedrockPort)

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%+v\n", resp)
}

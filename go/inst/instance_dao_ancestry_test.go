package inst

import (
	"testing"

	test "github.com/proxysql/golib/tests"
)

const (
	ancestryTestSourceUUID       = "623335cf-c858-3eb6-a90d-40122a2d21f4"
	ancestryTestIntermediateUUID = "00000000-0000-0000-0000-000000000200"
	ancestryTestLeafUUID         = "00000000-0000-0000-0000-000000000201"
)

// Topology from https://github.com/ProxySQL/orchestrator/issues/137:
//
//	source (e.g. AWS RDS, not readable by orchestrator: no backend row, no ancestry)
//	 └── intermediate
//	      └── leaf
//
// Returns the ancestry of the intermediate and the leaf as orchestrator computes them during discovery.
func ancestryTestChainWithUnreadableSource() (intermediateAncestry, leafAncestry string) {
	// The source has no row in the backend, so the intermediate's master ancestry is empty.
	intermediateAncestry = computeAncestryUUID("", ancestryTestSourceUUID, ancestryTestIntermediateUUID, "", false)
	leafAncestry = computeAncestryUUID(intermediateAncestry, ancestryTestIntermediateUUID, ancestryTestLeafUUID, "", false)
	return intermediateAncestry, leafAncestry
}

func TestComputeAncestryUUIDIncludesMasterUUID(t *testing.T) {
	intermediateAncestry, leafAncestry := ancestryTestChainWithUnreadableSource()

	intermediate := &Instance{ServerUUID: ancestryTestIntermediateUUID, AncestryUUID: intermediateAncestry}
	leaf := &Instance{ServerUUID: ancestryTestLeafUUID, AncestryUUID: leafAncestry}
	source := &Instance{ServerUUID: ancestryTestSourceUUID}

	test.S(t).ExpectTrue(intermediate.IsDescendantOf(source))
	test.S(t).ExpectTrue(leaf.IsDescendantOf(source))
	test.S(t).ExpectTrue(leaf.IsDescendantOf(intermediate))
	test.S(t).ExpectFalse(intermediate.IsDescendantOf(leaf))
}

func TestComputeAncestryUUIDNoDuplicates(t *testing.T) {
	// Fully discovered chain: the master's stored ancestry already contains the master's UUID.
	sourceAncestry := computeAncestryUUID("", "", ancestryTestSourceUUID, "", false)
	test.S(t).ExpectEquals(sourceAncestry, ancestryTestSourceUUID)

	intermediateAncestry := computeAncestryUUID(sourceAncestry, ancestryTestSourceUUID, ancestryTestIntermediateUUID, "", false)
	test.S(t).ExpectEquals(intermediateAncestry, ancestryTestSourceUUID+","+ancestryTestIntermediateUUID)
}

func TestComputeAncestryUUIDCoMaster(t *testing.T) {
	ancestry := computeAncestryUUID("whatever-stored", ancestryTestIntermediateUUID, ancestryTestLeafUUID, "", true)
	test.S(t).ExpectEquals(ancestry, ancestryTestIntermediateUUID+","+ancestryTestLeafUUID)
}

func TestComputeAncestryUUIDReplicationGroup(t *testing.T) {
	groupName := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	ancestry := computeAncestryUUID("", "", ancestryTestLeafUUID, groupName, false)
	test.S(t).ExpectEquals(ancestry, ancestryTestLeafUUID+","+groupName)
}

// The leaf has applied more of the source's transactions than the intermediate's last stored
// executed_gtid_set shows. None of them may be considered errant (issue #137).
func TestErrantCheckIgnoresUpstreamGtidsWithUnreadableSource(t *testing.T) {
	_, leafAncestry := ancestryTestChainWithUnreadableSource()

	leafExecuted := ancestryTestSourceUUID + ":1-4335836," + ancestryTestIntermediateUUID + ":1-5"
	redacted := redactExecutedGtidSetForErrantCheck(leafExecuted, leafAncestry, ancestryTestLeafUUID, false)
	test.S(t).ExpectTrue(redacted.IsEmpty())
}

// Transactions written on the replica itself must still be reported as errant.
func TestErrantCheckKeepsLocalGtids(t *testing.T) {
	intermediateAncestry, leafAncestry := ancestryTestChainWithUnreadableSource()

	leafExecuted := ancestryTestSourceUUID + ":1-4335836," + ancestryTestLeafUUID + ":1-3"
	redacted := redactExecutedGtidSetForErrantCheck(leafExecuted, leafAncestry, ancestryTestLeafUUID, false)
	test.S(t).ExpectEquals(redacted.String(), ancestryTestLeafUUID+":1-3")

	intermediateExecuted := ancestryTestSourceUUID + ":1-4335836," + ancestryTestIntermediateUUID + ":1-7"
	redacted = redactExecutedGtidSetForErrantCheck(intermediateExecuted, intermediateAncestry, ancestryTestIntermediateUUID, false)
	test.S(t).ExpectEquals(redacted.String(), ancestryTestIntermediateUUID+":1-7")
}

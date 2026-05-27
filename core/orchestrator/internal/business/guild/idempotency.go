package guild

// V22-style operation keys for the two money-moving guild flows.
// Mirrors wallet.OpType so reviewers see the same shape.
//
// accept_bid:<bidID>          — fired by AcceptBid; folds the wallet
//                               Debit + the Payout record + the task
//                               status transition into one logical
//                               unit a Temporal retry can re-drive.
// install_template:<installID>— fired by InstallTemplate; folds the
//                               wallet Debit + the Install record + the
//                               template install_count bump into one
//                               unit.

// OpType is the closed enum of guild operation kinds. Stored verbatim
// in guild_operations.op_type; new values land in both this file and
// the CHECK constraint in 000XX_guild.sql.
type OpType string

const (
	OpAcceptBid       OpType = "accept_bid"
	OpInstallTemplate OpType = "install_template"
	OpRejectTask      OpType = "reject_task"
	OpExpireTask      OpType = "expire_task"
)

// AcceptBidOpKey returns the deterministic op_key for AcceptBid. A
// Temporal retry that re-drives the same bid acceptance hits the same
// key and short-circuits via Service.RecallOp.
func AcceptBidOpKey(bidID string) string { return string(OpAcceptBid) + ":" + bidID }

// InstallTemplateOpKey returns the deterministic op_key for
// InstallTemplate. Keyed by (templateID, projectID) so a single
// project installing the same template twice is treated as ONE
// install, not two — install_count must reflect projects, not retries.
func InstallTemplateOpKey(templateID, projectID string) string {
	return string(OpInstallTemplate) + ":" + templateID + ":" + projectID
}

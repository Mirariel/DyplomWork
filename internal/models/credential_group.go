package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// CredentialGroup — іменована група API ключів користувача.
type CredentialGroup struct {
	ID        int64     `db:"id"         json:"id"`
	UserID    int64     `db:"user_id"    json:"user_id"`
	Name      string    `db:"name"       json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// CredentialGroupWithMembers — група з переліком credential_id.
type CredentialGroupWithMembers struct {
	CredentialGroup
	MemberIDs []int64 `json:"member_ids"`
}

// CredentialGroupRepository — DB-операції для груп API ключів.
type CredentialGroupRepository struct {
	db *sqlx.DB
}

func NewCredentialGroupRepository(db *sqlx.DB) *CredentialGroupRepository {
	return &CredentialGroupRepository{db: db}
}

// Create створює нову групу.
func (r *CredentialGroupRepository) Create(g *CredentialGroup) error {
	res, err := r.db.Exec(
		`INSERT INTO credential_groups (user_id, name) VALUES (?, ?)`,
		g.UserID, g.Name,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	g.ID = id
	return nil
}

// ListByUser повертає всі групи користувача з member_ids.
func (r *CredentialGroupRepository) ListByUser(userID int64) ([]CredentialGroupWithMembers, error) {
	groups := make([]CredentialGroup, 0)
	if err := r.db.Select(&groups,
		`SELECT * FROM credential_groups WHERE user_id = ? ORDER BY name`, userID); err != nil {
		return nil, err
	}

	result := make([]CredentialGroupWithMembers, len(groups))
	for i, g := range groups {
		result[i].CredentialGroup = g

		var memberIDs []int64
		if err := r.db.Select(&memberIDs,
			`SELECT credential_id FROM credential_group_members WHERE group_id = ?`, g.ID); err != nil {
			return nil, err
		}
		if memberIDs == nil {
			memberIDs = []int64{}
		}
		result[i].MemberIDs = memberIDs
	}
	return result, nil
}

// Delete видаляє групу (каскадно видаляє members).
func (r *CredentialGroupRepository) Delete(id, userID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM credential_groups WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// AddMember додає credential до групи.
func (r *CredentialGroupRepository) AddMember(groupID, credentialID int64) error {
	_, err := r.db.Exec(
		`INSERT IGNORE INTO credential_group_members (group_id, credential_id) VALUES (?, ?)`,
		groupID, credentialID)
	return err
}

// RemoveMember видаляє credential з групи.
func (r *CredentialGroupRepository) RemoveMember(groupID, credentialID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM credential_group_members WHERE group_id = ? AND credential_id = ?`,
		groupID, credentialID)
	return err
}

// SetMembers замінює всіх учасників групи на новий перелік.
func (r *CredentialGroupRepository) SetMembers(groupID int64, credentialIDs []int64) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM credential_group_members WHERE group_id = ?`, groupID); err != nil {
		return err
	}
	for _, cid := range credentialIDs {
		if _, err := tx.Exec(
			`INSERT INTO credential_group_members (group_id, credential_id) VALUES (?, ?)`,
			groupID, cid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

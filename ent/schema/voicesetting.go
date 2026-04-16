package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/disgoorg/snowflake/v2"
)

// VoiceSetting holds the schema definition for the VoiceSetting entity.
type VoiceSetting struct {
	ent.Schema
}

// Fields of the VoiceSetting.
func (VoiceSetting) Fields() []ent.Field {
	return []ent.Field{
		field.Uint64("user_id").
			GoType(snowflake.ID(0)).
			Unique().
			Immutable(),
		field.String("voice_name").
			Nillable().
			Optional(),
		field.Float("speaking_rate").
			Nillable().
			Optional(),
		field.Float("pitch").
			Nillable().
			Optional(),
	}
}

// Edges of the VoiceSetting.
func (VoiceSetting) Edges() []ent.Edge {
	return nil
}

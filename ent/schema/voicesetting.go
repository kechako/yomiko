package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// VoiceSetting holds the schema definition for the VoiceSetting entity.
type VoiceSetting struct {
	ent.Schema
}

// Fields of the VoiceSetting.
func (VoiceSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").
			Unique().
			NotEmpty().
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

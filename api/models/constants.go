package models

var Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var ValidCategories = map[VotingCategory]string{
	CategoryGrandJury:     "grand_jury",
	CategoryOtherTeam:     "other_team",
	CategoryGeneralPublic: "general_public",
}

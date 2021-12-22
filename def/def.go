package Def

type Poker struct {
	Num  byte // 2~10, 11: J, 12:Q, 13:K, 14:A
	Kind byte // 1:Plums , 2:Diamond, 3:Heart, 4:Spade
}

type Cards [7]Poker // 第0~1為手牌, 第2~6為公牌

/*
桌子可額外提供的
桌子狀態、莊家位、小盲注位、大盲注位、是否升盲(獎金賽用)
*/

type RobotInherit struct {
	ContestMoney float64 // Total money on table
	BetPos       float64 // Current self total bet
	Card         Cards   // 第0~1為手牌, 第2~6為公牌
	SbBet        float64 // Small blind
	RaiseCounter byte    // number of raise this round (含其他玩家的加注)
	RaiseSelf    byte    // number of raise only self (只算自己)
	RaisebyOther bool    // number of raise one round before except self
	IsContest    bool    // 是否為獎金賽
	PlayerNum    byte    // Number of player this round
}

type PlayerAction uint8

const (
	PLAYER_ACTION_CALL  PlayerAction = 0x01 // 跟注/call
	PLAYER_ACTION_CHECK PlayerAction = 0x02 // 過牌/check
	PLAYER_ACTION_RAISE PlayerAction = 0x04 // 加注/raise
	PLAYER_ACTION_FOLD  PlayerAction = 0x08 // 放棄/fold
	PLAYER_ACTION_ALLIN PlayerAction = 0x10 // 全下/all in
)

type SeatIndex struct {
	num int
}
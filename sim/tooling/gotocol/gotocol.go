package gotocol

type Message struct {
	Type         Type
	ResponseChan chan Message
	Payload      Payload
}

func (m Message) Send(to chan<- Message) {
	if to != nil {
		to <- m
	}
}

func (m Message) GoSend(to chan<- Message) {
	go func(c chan<- Message, msg Message) {
		if c != nil {
			c <- msg
		}
	}(to, m)
}

type Payload interface {
}

type Type int

const (
	// Hello - Controller Channel - actor name (string)
	Hello Type = iota
	// NameDrop - Buddy Channel - buddy name (string)
	NameDrop
	// JoinWorld - GameWorld Channel - player info
	JoinWorld
	// Goodbye - nil - sender name (string)
	Goodbye
)

func (t Type) String() string {
	switch t {
	case Hello:
		return "Hello"
	case NameDrop:
		return "NameDrop"
	case JoinWorld:
		return "JoinWorld"
	case Goodbye:
		return "Goodbye"
	default:
		return "Unknown"
	}
}

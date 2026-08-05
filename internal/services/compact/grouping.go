package compact

type MessageGroup struct {
	Role       string
	Messages   []Message
	StartIndex int
	EndIndex   int
}

func GroupByRole(messages []Message) []MessageGroup {
	if len(messages) == 0 {
		return nil
	}

	groups := make([]MessageGroup, 0)
	current := MessageGroup{
		Role:       messages[0].Role,
		Messages:   []Message{messages[0]},
		StartIndex: 0,
		EndIndex:   0,
	}

	for i := 1; i < len(messages); i++ {
		if messages[i].Role == current.Role {
			current.Messages = append(current.Messages, messages[i])
			current.EndIndex = i
		} else {
			groups = append(groups, current)
			current = MessageGroup{
				Role:       messages[i].Role,
				Messages:   []Message{messages[i]},
				StartIndex: i,
				EndIndex:   i,
			}
		}
	}

	groups = append(groups, current)
	return groups
}

func GroupByTurn(messages []Message, maxGroupSize int) []MessageGroup {
	if len(messages) == 0 {
		return nil
	}

	if maxGroupSize <= 0 {
		maxGroupSize = 10
	}

	groups := make([]MessageGroup, 0)
	current := MessageGroup{
		Role:       messages[0].Role,
		Messages:   make([]Message, 0, maxGroupSize),
		StartIndex: 0,
		EndIndex:   0,
	}

	for i, msg := range messages {
		if len(current.Messages) >= maxGroupSize && msg.Role != current.Role {
			groups = append(groups, current)
			current = MessageGroup{
				Role:       msg.Role,
				Messages:   make([]Message, 0, maxGroupSize),
				StartIndex: i,
				EndIndex:   i,
			}
		}
		current.Messages = append(current.Messages, msg)
		current.EndIndex = i
	}

	groups = append(groups, current)
	return groups
}

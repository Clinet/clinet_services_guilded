package guilded

import (
	"github.com/Clinet/clinet_cmds"
	"github.com/Clinet/clinet_convos"
	"github.com/Clinet/clinet_services"
)

func convoHandler(message *services.Message, session *ClientGuilded) (cmdResps []*cmds.CmdResp, err error) {
	if message == nil {
		return nil, cmds.ErrCmdEmptyMsg
	}
	content := message.Content
	if content == "" {
		return nil, nil
	}

	cmdResps = make([]*cmds.CmdResp, 0)

	conversation := convos.NewConversation()
	if oldConversation, err := Guilded.Storage.ServerGet(message.ServerID, "conversations_" + message.UserID); err == nil {
                switch oldConversation.(type) {
                        case convos.Conversation:
                                conversation = oldConversation.(convos.Conversation)
                        default:
                                Log.Trace("Skipping broken conversation record")
                }
        }
	conversationState := conversation.QueryText(content)
	if len(conversationState.Errors) > 0 {
		for _, csErr := range conversationState.Errors {
			Log.Error(csErr)
		}
	}
	if conversationState.Response != nil {
		//TODO: Dynamically build either an embed response or a simple conversation response
		Guilded.Storage.ServerSet(message.ServerID, "conversations_" + message.UserID, conversation)
		cmdResps = append(cmdResps, cmds.NewCmdRespMsg(conversationState.Response.TextSimple))
	} else {
		//TODO: Make a nice error message for failed queries in a conversation
		Guilded.Storage.ServerDel(message.ServerID, "conversations_" + message.UserID)
		cmdResps = append(cmdResps, cmds.NewCmdRespMsg("Erm... well this is awkward. I don't have an answer for that."))
	}

	return cmdResps, nil
}

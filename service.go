package guilded

import (
	"fmt"

	"github.com/Clinet/clinet_features"
	"github.com/Clinet/clinet_services"
	"github.com/Clinet/clinet_storage"
	"github.com/JoshuaDoes/guildrone"
	"github.com/JoshuaDoes/logger"
)

var Feature = features.Feature{
	Name: "guilded",
	ServiceChat: &ClientGuilded{},
}
var Guilded *ClientGuilded

var Log *logger.Logger
func init() {
	Log = logger.NewLogger("guilded", 2)
}

//ClientGuilded implements services.Service and holds a Guilded session
type ClientGuilded struct {
	*guildrone.Session
	cmdPrefix string
	botName   string
	User      guildrone.BotUser
	Cfg       *storage.Storage
	Storage   *storage.Storage
}

func (guilded *ClientGuilded) Shutdown() {
	_ = guilded.Close()
}

func (guilded *ClientGuilded) CmdPrefix() string {
	return guilded.cmdPrefix
}

func (guilded *ClientGuilded) Login() (err error) {
	Log.Trace("--- StartGuilded() ----")
	cfg := &storage.Storage{}
	if err := cfg.LoadFrom("guilded"); err != nil {
		return err
	}
	token, err := cfg.ConfigGet("cfg", "token")
	if err != nil {
		return err
	}
	cmdPrefix, err := cfg.ConfigGet("cfg", "cmdPrefix")
	if err != nil {
		return err
	}
	botName, err := cfg.ConfigGet("cfg", "botName")
	if err != nil {
		return err
	}

	state := &storage.Storage{}
	if err := state.LoadFrom("guildedstate"); err != nil {
		return err
	}

	Log.Debug("Creating Guilded struct...")
	guildedClient, err := guildrone.New(token.(string))
	if err != nil {
		return err
	}

	Log.Info("Registering Guilded event handlers...")
	guildedClient.AddHandler(guildedReady) //Not working yet?
	guildedClient.AddHandler(guildedChatMessageCreated)

	Log.Info("Connecting to Guilded...")
	err = guildedClient.Open()
	if err != nil {
		return err
	}

	Log.Info("Connected to Guilded!")
	Guilded = &ClientGuilded{guildedClient, cmdPrefix.(string), botName.(string), guildrone.BotUser{}, cfg, state}
	guilded = Guilded

	return nil
}

func (guilded *ClientGuilded) MsgEdit(msg *services.Message) (ret *services.Message, err error) {
	return nil, nil
}
func (guilded *ClientGuilded) MsgRemove(msg *services.Message) (err error) {
	return nil
}
func (guilded *ClientGuilded) MsgSend(msg *services.Message, ref interface{}) (ret *services.Message, err error) {
	if msg.ChannelID == "" {
		return nil, services.Error("guilded: MsgSend(msg: %v): missing channel ID", msg)
	}

	isPrivate := false
	if msg.ServerID == "" {
		isPrivate = true
	}

	var guildedMsg *guildrone.ChatMessage
	if msg.Title != "" || msg.Color > 0 || msg.Image != "" {
		retEmbed := guildrone.ChatEmbed{Description: msg.Content}
		if msg.Title != "" {
			retEmbed.Title = msg.Title
		}
		if msg.Color > 0 {
			retEmbed.Color = msg.Color
		}
		if msg.Image != "" {
			retEmbed.Image = &guildrone.ChatEmbedImage{
				URL: msg.Image,
			}
		}

		mc := &guildrone.MessageCreate{Embeds: []guildrone.ChatEmbed{retEmbed}}
		if isPrivate {
			mc.IsPrivate = true
		}
		guildedMsg, err = guilded.ChannelMessageCreateComplex(msg.ChannelID, mc)
	} else {
		if msg.Content == "" {
			return nil, services.Error("guilded: MsgSend(msg: %v): missing content", msg)
		}

		mc := &guildrone.MessageCreate{Content: msg.Content}
		if isPrivate {
			mc.IsPrivate = true
		}
		guildedMsg, err = guilded.ChannelMessageCreateComplex(msg.ChannelID, mc)
	}
	if err != nil {
		return nil, err
	}

	if guildedMsg != nil {
		ret = &services.Message{
			UserID: guildedMsg.CreatedBy,
			MessageID: guildedMsg.ID,
			ChannelID: guildedMsg.ChannelID,
			ServerID: guildedMsg.ServerID,
			Content: guildedMsg.Content,
			Context: guildedMsg,
		}
	}
	return ret, err
}

func (guilded *ClientGuilded) GetUser(serverID, userID string) (ret *services.User, err error) {
	user, err := guilded.ServerMemberGet(serverID, userID)
	if err != nil {
		return nil, err
	}
	userRoles := make([]*services.Role, len(user.RoleIds))
	for i := 0; i < len(userRoles); i++ {
		userRoles[i] = &services.Role{
			RoleID: fmt.Sprintf("%d", user.RoleIds[i]),
		}
	}
	return &services.User{
		ServerID: serverID,
		UserID: userID,
		Username: user.User.Name,
		Nickname: user.Nickname,
		Roles: userRoles,
	}, nil
}
func (guilded *ClientGuilded) GetUserPerms(serverID, channelID, userID string) (perms *services.Perms, err error) {
	server, err := guilded.GetServer(serverID)
	if err != nil {
		return nil, err
	}

	perms = &services.Perms{}
	//TODO: Permission mapping from Guilded

	if server.OwnerID == userID {
		perms.Administrator = true
	}

	return perms, nil
}
func (guilded *ClientGuilded) UserBan(user *services.User, reason string, rule int) (err error) {
	Log.Trace("Ban(", user.ServerID, ", ", user.UserID, ", ", reason, ", ", rule, ")")
	_, err = guilded.ServerMemberBanCreate(user.ServerID, user.UserID, reason)
	return err
}
func (guilded *ClientGuilded) UserKick(user *services.User, reason string, rule int) (err error) {
	Log.Trace("Kick(", user.ServerID, ", ", user.UserID, ", ", reason, ", ", rule, ")")
	return guilded.ServerMemberKick(user.ServerID, user.UserID)
}

func (guilded *ClientGuilded) GetServer(serverID string) (server *services.Server, err error) {
	srv, err := guilded.ServerGet(serverID)
	if err != nil {
		return nil, err
	}
	return &services.Server{
		ServerID: serverID,
		Name: srv.Name,
		Region: srv.Timezone,
		OwnerID: srv.OwnerID,
		DefaultChannel: srv.DefaultChannelID,
		VoiceStates: make([]*services.VoiceState, 0), //TODO: Voice states from Guilded
	}, nil
}

func (guilded *ClientGuilded) VoiceJoin(serverID, channelID string, muted, deafened bool) (err error) {
	return services.Error("guilded: VoiceJoin: stub")
}
func (guilded *ClientGuilded) VoiceLeave(serverID string) (err error) {
	return services.Error("guilded: VoiceLeave: stub")
}

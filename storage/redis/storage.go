package redis

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/tgo-team/tgo-talk/tgo"
	"strconv"
)

func init() {
	tgo.RegistryStorage(func(context *tgo.Context) tgo.Storage {
		return NewStorage(context)
	})
}

type Storage struct {
	storageMsgChan chan *tgo.MsgContext
	client         *redis.Client
	ctx *tgo.Context
	cacheChannelClientMap map[uint64][]uint64
}

func NewStorage(ctx *tgo.Context) *Storage {
	return &Storage{
		storageMsgChan: make(chan *tgo.MsgContext, 0),
		ctx:ctx,
		cacheChannelClientMap: map[uint64][]uint64{},
		client: redis.NewClient(&redis.Options{
			Addr:     "127.0.0.1:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
	}
}

func (s *Storage) StorageMsgChan() chan *tgo.MsgContext {
	return s.storageMsgChan
}

func (s *Storage) AddMsg(msgContext *tgo.MsgContext) error {
	msg := msgContext.Msg()
	sMsgID := fmt.Sprintf("%d", msg.MessageID)
	sChannelID := fmt.Sprintf("%d", msgContext.ChannelID())
	_, err := s.client.Set(sMsgID, msg, 0).Result()
	if err != nil {
		return err
	}
	_, err = s.client.LPush(fmt.Sprintf("ch_msg_list:%s", sChannelID), sMsgID).Result()
	if err != nil {
		return err
	}
	s.storageMsgChan <- msgContext
	return nil
}

func (s *Storage) GetMsg(msgID uint64) (*tgo.Msg, error) {
	key := fmt.Sprintf("%d", msgID)
	msg := &tgo.Msg{}
	err := s.client.Get(key).Scan(msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Storage) AddChannel(c *tgo.Channel) error {
	key := s.getChanelCacheKey(c.ChannelID)
	err := s.client.HMSet(key, map[string]interface {
	}{
		"channel_id":   fmt.Sprintf("%d",c.ChannelID),
		"channel_type": fmt.Sprintf("%d", c.ChannelType),
	}).Err()
	if err != nil {
		return err
	}
	return err
}
func (s *Storage) GetChannel(channelID uint64) (*tgo.Channel, error) {
	key := s.getChanelCacheKey(channelID)
	channelFieldMap, err := s.client.HGetAll(key).Result()
	if err != nil {
		return nil, err
	}
	sChannelID := channelFieldMap["channel_id"]
	if sChannelID =="" {
		return nil,nil
	}
	sChannelType := channelFieldMap["channel_type"]
	if sChannelType == "" {
		return nil,fmt.Errorf("channel[%v]类型不存在！",sChannelID)
	}
	chID, err := strconv.ParseInt(sChannelID, 10, 64)
	if err != nil {
		return nil, err
	}
	chType, err := strconv.ParseInt(sChannelType, 10, 64)
	if err != nil {
		return nil, err
	}
	ch := tgo.NewChannel(uint64(chID), tgo.ChannelType(chType),s.ctx)
	return ch, nil
}

func (s *Storage) AddClient(c *tgo.Client) error {

	return s.client.Set(s.getClientsCacheKey(c.ClientID), c, 0).Err()
}

func (s *Storage) Bind(clientID uint64, channelID uint64) error {

	return s.client.ZAdd(s.getChannelClientCacheKey(channelID), redis.Z{Score: 1.0, Member: clientID}).Err()
}

func (s *Storage) GetClientIDs(channelID uint64) ([]uint64 ,error) {
	clientIDs := make( []uint64,0)
	err := s.client.ZRange(s.getChannelClientCacheKey(channelID),0,10000).ScanSlice(&clientIDs)
	return clientIDs,err
}

func (s *Storage) GetClient(clientID uint64) (*tgo.Client,error) {
	client := &tgo.Client{}
	err := s.client.Get(s.getClientsCacheKey(clientID)).Scan(client)
	if err == redis.Nil {
		return nil,nil
	}
	return client,err
}

func (s *Storage) getChannelClientCacheKey(channelID uint64) string  {
	return fmt.Sprintf("%s%d","ch_c:", channelID)
}
func (s *Storage) getClientsCacheKey(clientID uint64)  string {
	return fmt.Sprintf("%s%d","c:", clientID)
}

func (s *Storage) getChanelCacheKey(channelID uint64)  string {
	return fmt.Sprintf("%s%d","ch:", channelID)
}

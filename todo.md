# 待办(代码深审遗留)

- [ ] message ID varint 解码补 8 字节上限:pkg/conn/message/message_encoder.go:150-159 的 varint 循环未强制 64 位(8 字节)上限,畸形报文可让 ID 解码吞掉整段负载;建议超过 8 个 continuation 字节即返回 ErrInvalidMessage(P3,2026-11-01 代码深审)

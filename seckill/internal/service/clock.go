package service

import "time"

// nowFunc 返回当前时间，可在测试中被替换以控制时间。
var nowFunc = time.Now

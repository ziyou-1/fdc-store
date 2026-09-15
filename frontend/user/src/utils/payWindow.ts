// 支付跳转统一走「新标签页」而不是整窗替换当前页。
//
// 采用「懒开」策略：调用 reservePayWindow() 时**不预先打开**任何标签，
// 只有真正拿到 redirect_url、调用 navigate(url) 时才 window.open。
//
// 为什么不预开空白标签：预开会在支付**失败**（如金额低于渠道最低限额被拒）时
// 先弹出一个空白标签、随即被 release() 关掉，用户看到「弹一下就关」的闪烁。
// 懒开则失败路径根本不开标签，无闪烁。
//
// 新标签能否成功打开取决于浏览器的用户手势有效期：支付接口通常在 1 秒内返回，
// 仍在点击手势的 transient activation 窗口内，window.open 不会被拦；极少数被拦时
// 退回当前标签跳转，保证支付不中断。
//
// 用法：
//   const payWindow = reservePayWindow()      // 同步调用（占位，暂不开标签）
//   try {
//     const res = await api...
//     if (res.redirect_url) { payWindow.navigate(res.redirect_url); return }
//     payWindow.release()                      // 不需要跳转时释放（当前为 no-op）
//   } catch (e) { payWindow.release(); ... }

export interface PendingPayWindow {
  navigate: (url: string) => void
  release: () => void
}

export function reservePayWindow(): PendingPayWindow {
  let navigated = false
  return {
    navigate(url: string) {
      if (navigated) return
      navigated = true
      let win: Window | null = null
      try {
        // 注意：不能传 'noopener'！带 noopener 时即使成功打开，window.open 也返回 null，
        // 会误判为被拦截而又执行下面的整窗跳转 —— 结果新标签开了、原标签也被替换成支付页
        // （用户看到「两个都是充值页、原网站没了」）。改为拿到窗口引用后手动切断 opener。
        win = window.open(url, '_blank')
        if (win) {
          try {
            win.opener = null
          } catch {
            /* 跨源置空失败可忽略 */
          }
        }
      } catch {
        win = null
      }
      // 仅当**确实**被弹窗拦截（返回 null）时才退回整窗跳转，保证支付仍能进行。
      if (!win) {
        window.location.href = url
      }
    },
    release() {
      // 懒开策略下没有预开的标签，无需清理。
      navigated = true
    },
  }
}

import { describe, expect, it } from 'vitest'
import { Editor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import { VideoNode } from './richEditorMedia'

// 复刻 RichEditor.vue 的关键构造：Image 先注册，Video 后注册。
// 顺序是 bug 的必要条件——后注册的扩展会覆盖同名命令。
const createEditor = () =>
  new Editor({
    extensions: [StarterKit, Image, VideoNode],
    content: '<p></p>',
  })

describe('富文本媒体节点', () => {
  it('上传图片必须插入 <img>，不能变成视频播放器', () => {
    const editor = createEditor()
    try {
      editor.chain().focus().setImage({ src: '/uploads/demo.png' }).run()
      const html = editor.getHTML()

      // 这就是用户看到的症状：上传的是图片，显示出来却是视频
      expect(html).not.toContain('<video')
      expect(html).toContain('<img')
      expect(html).toContain('/uploads/demo.png')
    } finally {
      editor.destroy()
    }
  })

  it('视频节点自身仍然渲染成 <video>', () => {
    const editor = createEditor()
    try {
      editor.commands.insertContent({ type: 'video', attrs: { src: '/uploads/clip.mp4' } })
      const html = editor.getHTML()

      expect(html).toContain('<video')
      expect(html).toContain('/uploads/clip.mp4')
    } finally {
      editor.destroy()
    }
  })

  it('已存的 <video> 标签仍能被解析回视频节点', () => {
    const editor = new Editor({
      extensions: [StarterKit, Image, VideoNode],
      content: '<video src="/uploads/old.mp4"></video>',
    })
    try {
      expect(editor.getHTML()).toContain('<video')
    } finally {
      editor.destroy()
    }
  })
})

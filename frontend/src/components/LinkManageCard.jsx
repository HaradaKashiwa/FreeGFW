import { useState } from "react"
import { useGetLinks, useCreateLink, useSwapLink, useDeleteLink } from "../apis/link"
import { Button } from "@/components/ui/button"
import { IoAddCircleOutline, IoLink, IoUnlink } from "react-icons/io5"
import { Input } from "@/components/ui/input"
import { PiSpinner } from "react-icons/pi"
import { Form } from "@/components/ui/form"
import { Modal } from "./Modal"

export function LinkManageCard() {
    const { data: links, loading: linksLoading, loaded: linksLoaded, refresh: refreshLinks } = useGetLinks()
    const { trigger: createLink, loading: createLinkLoading } = useCreateLink()
    const { trigger: swapLink, loading: swapLinkLoading } = useSwapLink()

    const [open, setOpen] = useState(false)
    const [inviteLink, setInviteLink] = useState('')

    const [preDeleteLink, setPreDeleteLink] = useState(null)
    const { trigger: deleteLink, loading: deleteLinkLoading } = useDeleteLink({ id: preDeleteLink?.id })

    const [error, setError] = useState(null)

    const handleCreateLink = async () => {
        const res = await createLink()
        if (res && res.link) {
            setInviteLink(res.link)
        }
    }

    const handleSwapLink = async (values) => {
        try {
            setError(null)
            await swapLink(values)
            refreshLinks()
            setOpen(false)
            setInviteLink('') // Reset
        } catch (e) {
            setError(e.message)
        }
    }

    return (
        <div className='bg-white rounded-lg pt-4 mb-4'>
            <div className='md:flex items-center justify-between px-4'>
                <div>
                    <div className='text-md'>连接到其他FreeGFW</div>
                    <div className='text-xs opacity-50'>管理与其他FreeGFW的链接</div>
                </div>
                <div className='flex items-center gap-2 mt-4 md:mt-0'>
                    <Input className='h-8' placeholder='搜索链接' />
                    <Modal
                        open={open}
                        onOpenChange={(val) => {
                            setOpen(val)
                            setError(null)
                            if (!val) setInviteLink('')
                        }}
                        title='添加链接'
                        description='生成邀请链接或者输入对方的链接进行连接'
                        content={
                            <div className="space-y-6 pt-2">
                                <div className="border p-4 rounded-lg bg-gray-50/50">
                                    <h3 className="font-semibold mb-3 text-sm">生成邀请</h3>
                                    <div className="flex gap-2">
                                        <Input value={inviteLink} readOnly placeholder="点击生成邀请链接" className="bg-white" />
                                        <Button onClick={handleCreateLink} disabled={createLinkLoading} className="whitespace-nowrap">
                                            {createLinkLoading ? <PiSpinner className="animate-spin" /> : "生成"}
                                        </Button>
                                    </div>
                                    {inviteLink && <div className="text-xs text-green-600 mt-2">已生成，请复制并发送给对方</div>}
                                </div>

                                <div className="border p-4 rounded-lg bg-gray-50/50">
                                    <h3 className="font-semibold mb-3 text-sm">连接对方</h3>
                                    <Form
                                        onSubmit={handleSwapLink}
                                        submitText="连接"
                                        submitLoading={swapLinkLoading}
                                        errors={error ? [{ field: 'link', message: error }] : []}
                                        fields={[
                                            {
                                                name: 'link',
                                                label: '对方链接',
                                                component: <Input name='link' placeholder="粘贴对方的链接" className="bg-white" />,
                                                description: '输入对方生成的邀请链接'
                                            }
                                        ]}
                                    />
                                </div>
                            </div>
                        }
                    >
                        <Button className='cursor-pointer' size='sm' onClick={() => setOpen(true)}>链接 <IoAddCircleOutline /></Button>
                    </Modal>
                </div>
            </div>
            <div className='mt-4'>
                <div className='flex items-center gap-4 p-4 py-2 font-bold border-b'>
                    <div className='flex-1 flex items-center gap-2'>
                        链接地址
                    </div>
                    <div className="flex gap-2">
                        操作
                    </div>
                </div>
                <div className='max-h-96 overflow-y-auto'>
                    {!linksLoaded && <PiSpinner className='text-primary animate-spin text-2xl mx-auto m-5' />}
                    {!links?.length && linksLoaded && <div className='text-center text-sm opacity-70 m-5'>暂无链接，开始添加一个链接吧</div>}
                    {links?.map(link => (
                        <div key={link.id} className='flex items-center gap-4 p-4 border-b last:border-b-0'>
                            <div className='flex-1 flex items-center gap-4 truncate text-sm text-gray-600' title={link.ip}>
                                <div className={`w-8 h-8 rounded-full bg-gray-100 flex items-center justify-center text-gray-400 ${link.lastSyncStatus === 'success' ? 'bg-green-100 text-green-600' : 'bg-red-100 text-red-600'}`}>
                                    {link.lastSyncStatus === 'success' ? <IoLink className="text-lg" /> : <IoUnlink className="text-lg rotate-45" />}
                                </div>
                                <div className='flex-1 truncate'>
                                    {(() => {
                                        if (link.lastSyncStatus === 'failed') return <span className="text-red-500">{link.error || '连接失败'}</span>;
                                        if (link.lastSyncStatus !== 'success') return '连接中';

                                        const title = link.name || link.server?.title || link.server?.name;
                                        const ip = link.ip || '未知 IP';

                                        return (
                                            <div>
                                                <div className="flex items-center gap-2">
                                                    <span className="font-medium text-gray-900">{title || ip}</span>
                                                    {title && <span className="text-gray-400 text-xs">({ip})</span>}
                                                </div>
                                                {link.lastSyncAt && (
                                                    <div className="text-xs text-gray-400 mt-0.5">
                                                        最后同步: {new Date(link.lastSyncAt * 1000).toLocaleString()}
                                                    </div>
                                                )}
                                            </div>
                                        );
                                    })()}
                                </div>
                            </div>
                            <div className="flex gap-2">
                                <Button size='sm' variant='destructive' className='cursor-pointer' onClick={() => setPreDeleteLink(link)}><IoUnlink /></Button>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
            <Modal
                title='断开连接'
                description={`确定要断开与此链接的连接吗？`}
                open={!!preDeleteLink}
                onOpenChange={() => setPreDeleteLink(null)}
                content={
                    <div className='flex gap-2 justify-end'>
                        <Button variant='outline' onClick={() => setPreDeleteLink(null)}>取消</Button>
                        <Button variant='destructive' onClick={async () => {
                            await deleteLink()
                            refreshLinks()
                            setPreDeleteLink(null)
                        }}>确定 {deleteLinkLoading && <PiSpinner className='animate-spin' />}</Button>
                    </div>
                }
            />
        </div>
    )
}

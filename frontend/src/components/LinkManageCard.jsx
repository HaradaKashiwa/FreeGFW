import { useState } from "react"
import { useGetLinks, useCreateLink, useSwapLink, useDeleteLink } from "../apis/link"
import { Button } from "@/components/ui/button"
import { IoAddCircleOutline, IoLink, IoUnlink } from "react-icons/io5"
import { Input } from "@/components/ui/input"
import { PiSpinner } from "react-icons/pi"
import { Form } from "@/components/ui/form"
import { Modal } from "./Modal"
import { useLanguageStore } from "../store/useLanguageStore"

export function LinkManageCard() {
    const { data: links, loading: linksLoading, loaded: linksLoaded, refresh: refreshLinks } = useGetLinks()
    const { trigger: createLink, loading: createLinkLoading } = useCreateLink()
    const { trigger: swapLink, loading: swapLinkLoading } = useSwapLink()

    const [open, setOpen] = useState(false)
    const [inviteLink, setInviteLink] = useState('')

    const [preDeleteLink, setPreDeleteLink] = useState(null)
    const { trigger: deleteLink, loading: deleteLinkLoading } = useDeleteLink({ id: preDeleteLink?.id })

    const [error, setError] = useState(null)
    const { t } = useLanguageStore()

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
                    <div className='text-md'>{t('connect_other_freegfw')}</div>
                    <div className='text-xs opacity-50'>{t('manage_links_desc')}</div>
                </div>
                <div className='flex items-center gap-2 mt-4 md:mt-0'>
                    <Input className='h-8' placeholder={t('search_links')} />
                    <Modal
                        open={open}
                        onOpenChange={(val) => {
                            setOpen(val)
                            setError(null)
                            if (!val) setInviteLink('')
                        }}
                        title={t('add_link_title')}
                        description={t('add_link_desc')}
                        content={
                            <div className="space-y-6 pt-2">
                                <div className="border p-4 rounded-lg bg-gray-50/50">
                                    <h3 className="font-semibold mb-3 text-sm">{t('generate_invite')}</h3>
                                    <div className="flex gap-2">
                                        <Input value={inviteLink} readOnly placeholder={t('click_generate_invite')} className="bg-white" />
                                        <Button onClick={handleCreateLink} disabled={createLinkLoading} className="whitespace-nowrap">
                                            {createLinkLoading ? <PiSpinner className="animate-spin" /> : t('generate')}
                                        </Button>
                                    </div>
                                    {inviteLink && <div className="text-xs text-green-600 mt-2">{t('generated_copy_send')}</div>}
                                </div>

                                <div className="border p-4 rounded-lg bg-gray-50/50">
                                    <h3 className="font-semibold mb-3 text-sm">{t('connect_peer')}</h3>
                                    <Form
                                        onSubmit={handleSwapLink}
                                        submitText={t('connect')}
                                        submitLoading={swapLinkLoading}
                                        errors={error ? [{ field: 'link', message: error }] : []}
                                        fields={[
                                            {
                                                name: 'link',
                                                label: t('peer_link'),
                                                component: <Input name='link' placeholder={t('paste_peer_link')} className="bg-white" />,
                                                description: t('peer_link_desc')
                                            }
                                        ]}
                                    />
                                </div>
                            </div>
                        }
                    >
                        <Button className='cursor-pointer' size='sm' onClick={() => setOpen(true)}>{t('add_link')} <IoAddCircleOutline /></Button>
                    </Modal>
                </div>
            </div>
            <div className='mt-4'>
                <div className='flex items-center gap-4 p-4 py-2 font-bold border-b'>
                    <div className='flex-1 flex items-center gap-2'>
                        {t('link_address')}
                    </div>
                    <div className="flex gap-2 text-end">
                        {t('actions')}
                    </div>
                </div>
                <div className='max-h-96 overflow-y-auto'>
                    {!linksLoaded && <PiSpinner className='text-primary animate-spin text-2xl mx-auto m-5' />}
                    {!links?.length && linksLoaded && <div className='text-center text-sm opacity-70 m-5'>{t('no_users_yet')}</div>}
                    {links?.map(link => (
                        <div key={link.id} className='flex items-center gap-4 p-4 border-b last:border-b-0'>
                            <div className='flex-1 flex items-center gap-4 truncate text-sm text-gray-600' title={link.ip}>
                                <div className={`w-8 h-8 rounded-full bg-gray-100 flex items-center justify-center text-gray-400 ${link.lastSyncStatus === 'success' ? 'bg-green-100 text-green-600' : 'bg-red-100 text-red-600'}`}>
                                    {link.lastSyncStatus === 'success' ? <IoLink className="text-lg" /> : <IoUnlink className="text-lg rotate-45" />}
                                </div>
                                <div className='flex-1 truncate'>
                                    {(() => {
                                        if (link.lastSyncStatus === 'failed') return <span className="text-red-500">{link.error || t('connection_failed')}</span>;
                                        if (link.lastSyncStatus !== 'success') return t('connecting');

                                        const title = link.name || link.server?.title || link.server?.name;
                                        const ip = link.ip || t('unknown_ip');

                                        return (
                                            <div>
                                                <div className="flex items-center gap-2">
                                                    <span className="font-medium text-gray-900">{title || ip}</span>
                                                    {title && <span className="text-gray-400 text-xs">({ip})</span>}
                                                </div>
                                                {link.lastSyncAt && (
                                                    <div className="text-xs text-gray-400 mt-0.5" dir="ltr">
                                                        {t('last_sync')}: {new Date(link.lastSyncAt * 1000).toLocaleString()}
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
                title={t('disconnect')}
                description={t('disconnect_confirm')}
                open={!!preDeleteLink}
                onOpenChange={() => setPreDeleteLink(null)}
                content={
                    <div className='flex gap-2 justify-end'>
                        <Button variant='outline' onClick={() => setPreDeleteLink(null)}>{t('cancel')}</Button>
                        <Button variant='destructive' onClick={async () => {
                            await deleteLink()
                            refreshLinks()
                            setPreDeleteLink(null)
                        }}>{t('confirm')} {deleteLinkLoading && <PiSpinner className='animate-spin' />}</Button>
                    </div>
                }
            />
        </div>
    )
}
